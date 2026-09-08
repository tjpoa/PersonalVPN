package postgres

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"personalvpn/services/api/internal/agent"
	"personalvpn/services/api/internal/control"
)

const operationTimeout = 5 * time.Second

type Store struct {
	pool *pgxpool.Pool
}

func (s *Store) ListRegions(ctx context.Context) ([]control.Region, error) {
	queryContext, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	rows, err := s.pool.Query(queryContext, `SELECT id, display_name, available FROM regions ORDER BY id`)
	if err != nil {
		return nil, mapDatabaseError(err)
	}
	defer rows.Close()
	regions := make([]control.Region, 0)
	for rows.Next() {
		var region control.Region
		if err := rows.Scan(&region.ID, &region.DisplayName, &region.Available); err != nil {
			return nil, fmt.Errorf("scan region: %w", err)
		}
		regions = append(regions, region)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list regions: %w", err)
	}
	return regions, nil
}

type SnapshotDelivery struct {
	ID         string
	GatewayID  string
	Generation int64
	Payload    []byte
	Signature  []byte
	Attempts   int
}

// GatewaysNeedingSnapshot returns ready gateways whose desired peer state is
// ahead of the last state acknowledged by the agent.
func (s *Store) GatewaysNeedingSnapshot(ctx context.Context) ([]string, error) {
	queryContext, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	rows, err := s.pool.Query(queryContext, `
		SELECT DISTINCT g.id::text
		FROM gateways g
		JOIN peer_assignments pa ON pa.gateway_id = g.id
		WHERE g.state = 'ready'
		  AND pa.state IN ('pending', 'active', 'removal_pending')
		  AND pa.desired_generation > pa.observed_generation
		  AND NOT EXISTS (
			SELECT 1 FROM gateway_snapshots gs
			WHERE gs.gateway_id = g.id
			  AND gs.state IN ('queued', 'in_flight')
		  )
		ORDER BY g.id::text`)
	if err != nil {
		return nil, mapDatabaseError(err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan gateway: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list gateways needing snapshot: %w", err)
	}
	return ids, nil
}

func (s *Store) NextGatewaySnapshotGeneration(ctx context.Context, gatewayID string) (int64, error) {
	if gatewayID == "" {
		return 0, control.ErrInvalidInput
	}
	queryContext, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	var generation int64
	err := s.pool.QueryRow(queryContext, `SELECT COALESCE(MAX(generation), 0) + 1 FROM gateway_snapshots WHERE gateway_id = $1::uuid`, gatewayID).Scan(&generation)
	return generation, mapDatabaseError(err)
}

// EnqueueCurrentGatewaySnapshot materializes assigned peers into a signed
// outbox delivery. The signing key comes from an external secret manager.
func (s *Store) EnqueueCurrentGatewaySnapshot(ctx context.Context, gatewayID string, generation int64, signingKey ed25519.PrivateKey) error {
	if gatewayID == "" || generation <= 0 || len(signingKey) != ed25519.PrivateKeySize {
		return control.ErrInvalidInput
	}
	queryContext, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	rows, err := s.pool.Query(queryContext, `
		SELECT d.public_key, host(l.ipv4), host(l.ipv6)
		FROM peer_assignments pa
		JOIN devices d ON d.id = pa.device_id
		JOIN ip_leases l ON l.id = pa.lease_id
		WHERE pa.gateway_id = $1::uuid AND pa.state IN ('pending', 'active', 'removal_pending')
		ORDER BY d.id`, gatewayID)
	if err != nil {
		return mapDatabaseError(err)
	}
	defer rows.Close()
	peers := make([]agent.Peer, 0)
	for rows.Next() {
		var publicKey, ipv4, ipv6 string
		if err := rows.Scan(&publicKey, &ipv4, &ipv6); err != nil {
			return err
		}
		peers = append(peers, agent.Peer{PublicKey: publicKey, IPv4: ipv4 + "/32", IPv6: ipv6 + "/128"})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	snapshot := agent.Snapshot{GatewayID: gatewayID, Generation: generation, ExpiresAt: time.Now().UTC().Add(10 * time.Minute), Peers: peers}
	signed, err := agent.Sign(snapshot, signingKey)
	if err != nil {
		return err
	}
	unsigned := signed
	unsigned.Signature = nil
	payload, err := json.Marshal(unsigned)
	if err != nil {
		return err
	}
	return s.EnqueueSnapshot(queryContext, gatewayID, generation, payload, signed.Signature)
}

// Gateway delivery adapters keep HTTP transport independent from PostgreSQL.
func (s *Store) ClaimSnapshot(ctx context.Context, gatewayID string) (agent.Delivery, error) {
	delivery, err := s.ClaimNextSnapshot(ctx, gatewayID)
	if err != nil {
		return agent.Delivery{}, err
	}
	return agent.Delivery{ID: delivery.ID, Generation: delivery.Generation, Payload: delivery.Payload, Signature: delivery.Signature}, nil
}

func (s *Store) AckSnapshotDelivery(ctx context.Context, gatewayID, deliveryID string, generation int64) error {
	if err := s.AckSnapshot(ctx, gatewayID, deliveryID, generation); err != nil {
		return err
	}
	return s.ConfirmGatewaySnapshot(ctx, gatewayID, generation)
}

func (s *Store) RetrySnapshotDelivery(ctx context.Context, gatewayID, deliveryID string, retryAt time.Time, reason string) error {
	return s.RetrySnapshot(ctx, gatewayID, deliveryID, retryAt, reason)
}

// ConfirmGatewaySnapshot advances assignment state only after the agent has
// acknowledged the corresponding generation.
func (s *Store) ConfirmGatewaySnapshot(ctx context.Context, gatewayID string, generation int64) error {
	if gatewayID == "" || generation <= 0 {
		return control.ErrInvalidInput
	}
	queryContext, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	tx, err := s.pool.BeginTx(queryContext, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin gateway confirmation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = tx.Exec(queryContext, `
		UPDATE peer_assignments
		SET state = CASE WHEN state = 'removal_pending' THEN 'removed'::assignment_state ELSE 'active'::assignment_state END,
		    observed_generation = desired_generation, updated_at = now()
		WHERE gateway_id = $1::uuid
		  AND desired_generation <= $2
		  AND state IN ('pending', 'active', 'removal_pending')`, gatewayID, generation)
	if err != nil {
		return mapDatabaseError(err)
	}
	if _, err = tx.Exec(queryContext, `
		UPDATE ip_leases l SET released_at = now()
		WHERE l.released_at IS NULL AND EXISTS (
			SELECT 1 FROM peer_assignments pa
			WHERE pa.lease_id = l.id AND pa.gateway_id = $1::uuid AND pa.state = 'removed'
		)`, gatewayID); err != nil {
		return mapDatabaseError(err)
	}
	if _, err = tx.Exec(queryContext, `
		UPDATE devices d SET state = 'revoked', revoked_at = now()
		WHERE d.state = 'revocation_pending' AND EXISTS (
			SELECT 1 FROM peer_assignments pa
			WHERE pa.device_id = d.id AND pa.gateway_id = $1::uuid AND pa.state = 'removed'
		)`, gatewayID); err != nil {
		return mapDatabaseError(err)
	}
	if err := tx.Commit(queryContext); err != nil {
		return fmt.Errorf("commit gateway confirmation: %w", err)
	}
	return nil
}

func (s *Store) Ping(ctx context.Context) error {
	queryContext, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	return s.pool.Ping(queryContext)
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}
	return openConfig(ctx, config)
}

// OpenWithPasswordFile injects a password from a mounted secret without placing
// it in DATABASE_URL, process listings, or logs.
func OpenWithPasswordFile(ctx context.Context, databaseURL, passwordFile string) (*Store, error) {
	password, err := os.ReadFile(passwordFile)
	if err != nil {
		return nil, fmt.Errorf("read database password: %w", err)
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}
	config.ConnConfig.Password = strings.TrimSpace(string(password))
	if config.ConnConfig.Password == "" {
		return nil, errors.New("database password is empty")
	}
	return openConfig(ctx, config)
}

func openConfig(ctx context.Context, config *pgxpool.Config) (*Store, error) {
	if err := requireTLSForRemoteHost(config); err != nil {
		return nil, err
	}
	config.MaxConns = 20
	config.MinConns = 2
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute
	config.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	store := &Store{pool: pool}
	pingContext, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	if err := pool.Ping(pingContext); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return store, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) CreateUser(ctx context.Context, normalizedEmail, passwordHash string) (string, error) {
	queryContext, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	var id string
	err := s.pool.QueryRow(queryContext, `
		INSERT INTO users (email_normalized, password_hash)
		VALUES ($1, $2)
		RETURNING id::text`, normalizedEmail, passwordHash).Scan(&id)
	if err != nil {
		return "", mapDatabaseError(err)
	}
	return id, nil
}

func (s *Store) FindUserPasswordHash(ctx context.Context, normalizedEmail string) (string, string, bool, error) {
	queryContext, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	var userID string
	var passwordHash string
	var disabledAt *time.Time
	err := s.pool.QueryRow(queryContext, `
		SELECT id::text, password_hash, disabled_at
		FROM users
		WHERE email_normalized = $1`, normalizedEmail).Scan(&userID, &passwordHash, &disabledAt)
	if err != nil {
		return "", "", false, mapDatabaseError(err)
	}
	return userID, passwordHash, disabledAt != nil, nil
}

func (s *Store) RegisterDevice(ctx context.Context, ownerID string, request control.RegisterDeviceRequest) (control.Device, error) {
	if err := control.ValidateRegisterDevice(ownerID, request); err != nil {
		return control.Device{}, err
	}
	queryContext, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	var device control.Device
	var platform string
	var state string
	err := s.pool.QueryRow(queryContext, `
		INSERT INTO devices (user_id, name, platform, public_key)
		VALUES ($1::uuid, $2, $3, $4)
		RETURNING id::text, name, platform::text, public_key, state::text, created_at`,
		ownerID, strings.TrimSpace(request.Name), request.Platform, request.PublicKey,
	).Scan(&device.ID, &device.Name, &platform, &device.PublicKey, &state, &device.CreatedAt)
	if err != nil {
		return control.Device{}, mapDatabaseError(err)
	}
	device.Platform = control.Platform(platform)
	device.State = control.DeviceState(state)
	return device, nil
}

func (s *Store) ListDevices(ctx context.Context, ownerID string) ([]control.Device, error) {
	queryContext, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	rows, err := s.pool.Query(queryContext, `
		SELECT id::text, name, platform::text, public_key, state::text, created_at
		FROM devices
		WHERE user_id = $1::uuid
		ORDER BY created_at, id`, ownerID)
	if err != nil {
		return nil, mapDatabaseError(err)
	}
	defer rows.Close()

	devices := make([]control.Device, 0)
	for rows.Next() {
		var device control.Device
		var platform string
		var state string
		if err := rows.Scan(&device.ID, &device.Name, &platform, &device.PublicKey, &state, &device.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		device.Platform = control.Platform(platform)
		device.State = control.DeviceState(state)
		devices = append(devices, device)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	return devices, nil
}

func (s *Store) RequestRevocation(ctx context.Context, ownerID, deviceID string) error {
	queryContext, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	var id string
	err := s.pool.QueryRow(queryContext, `
		UPDATE devices
		SET state = CASE WHEN state = 'revoked' THEN state ELSE 'revocation_pending' END
		WHERE id = $1::uuid AND user_id = $2::uuid
		RETURNING id::text`, deviceID, ownerID).Scan(&id)
	return mapDatabaseError(err)
}

func (s *Store) EnqueueSnapshot(ctx context.Context, gatewayID string, generation int64, payload, signature []byte) error {
	if strings.TrimSpace(gatewayID) == "" || generation <= 0 || len(payload) == 0 || len(signature) != 64 {
		return control.ErrInvalidInput
	}
	queryContext, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	_, err := s.pool.Exec(queryContext, `
		INSERT INTO gateway_snapshots (gateway_id, generation, payload, signature)
		VALUES ($1::uuid, $2, $3::jsonb, $4)`, gatewayID, generation, payload, signature)
	return mapDatabaseError(err)
}

func (s *Store) ClaimNextSnapshot(ctx context.Context, gatewayID string) (SnapshotDelivery, error) {
	if strings.TrimSpace(gatewayID) == "" {
		return SnapshotDelivery{}, control.ErrInvalidInput
	}
	queryContext, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	tx, err := s.pool.BeginTx(queryContext, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return SnapshotDelivery{}, fmt.Errorf("begin snapshot claim: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var delivery SnapshotDelivery
	err = tx.QueryRow(queryContext, `
		SELECT id::text, gateway_id::text, generation, payload, signature, attempts
		FROM gateway_snapshots
		WHERE gateway_id = $1::uuid
		  AND state IN ('queued', 'failed')
		  AND available_at <= now()
		  AND attempts < 100
		ORDER BY generation
		LIMIT 1
		FOR UPDATE SKIP LOCKED`, gatewayID).Scan(
		&delivery.ID, &delivery.GatewayID, &delivery.Generation,
		&delivery.Payload, &delivery.Signature, &delivery.Attempts,
	)
	if err != nil {
		return SnapshotDelivery{}, mapDatabaseError(err)
	}
	err = tx.QueryRow(queryContext, `
		UPDATE gateway_snapshots
		SET state = 'in_flight', attempts = attempts + 1, sent_at = now()
		WHERE id = $1::uuid
		RETURNING attempts`, delivery.ID).Scan(&delivery.Attempts)
	if err != nil {
		return SnapshotDelivery{}, mapDatabaseError(err)
	}
	if err := tx.Commit(queryContext); err != nil {
		return SnapshotDelivery{}, fmt.Errorf("commit snapshot claim: %w", err)
	}
	return delivery, nil
}

func (s *Store) AckSnapshot(ctx context.Context, gatewayID, snapshotID string, generation int64) error {
	if strings.TrimSpace(gatewayID) == "" || strings.TrimSpace(snapshotID) == "" || generation <= 0 {
		return control.ErrInvalidInput
	}
	queryContext, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	commandTag, err := s.pool.Exec(queryContext, `
		UPDATE gateway_snapshots
		SET state = 'acked', acked_at = now()
		WHERE id = $1::uuid AND gateway_id = $2::uuid
		  AND generation = $3 AND state = 'in_flight'`, snapshotID, gatewayID, generation)
	if err != nil {
		return mapDatabaseError(err)
	}
	if commandTag.RowsAffected() != 1 {
		return control.ErrNotFound
	}
	return nil
}

func (s *Store) RetrySnapshot(ctx context.Context, gatewayID, snapshotID string, retryAt time.Time, reason string) error {
	if strings.TrimSpace(gatewayID) == "" || strings.TrimSpace(snapshotID) == "" || retryAt.IsZero() {
		return control.ErrInvalidInput
	}
	if len(reason) > 512 {
		reason = reason[:512]
	}
	queryContext, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	commandTag, err := s.pool.Exec(queryContext, `
		UPDATE gateway_snapshots
		SET state = 'failed', available_at = $3, last_error = $4
		WHERE id = $1::uuid AND gateway_id = $2::uuid AND state = 'in_flight'`, snapshotID, gatewayID, retryAt, reason)
	if err != nil {
		return mapDatabaseError(err)
	}
	if commandTag.RowsAffected() != 1 {
		return control.ErrNotFound
	}
	return nil
}

func (s *Store) CreateTunnelConfiguration(
	ctx context.Context,
	ownerID string,
	deviceID string,
	regionID string,
	idempotencyKey string,
) (control.TunnelConfiguration, bool, error) {
	if strings.TrimSpace(ownerID) == "" || strings.TrimSpace(deviceID) == "" || strings.TrimSpace(regionID) == "" {
		return control.TunnelConfiguration{}, false, control.ErrInvalidInput
	}
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
		return control.TunnelConfiguration{}, false, control.ErrInvalidInput
	}

	queryContext, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	tx, err := s.pool.BeginTx(queryContext, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return control.TunnelConfiguration{}, false, fmt.Errorf("begin allocation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var deviceState string
	err = tx.QueryRow(queryContext, `
		SELECT state::text
		FROM devices
		WHERE id = $1::uuid AND user_id = $2::uuid
		FOR UPDATE`, deviceID, ownerID).Scan(&deviceState)
	if err != nil {
		return control.TunnelConfiguration{}, false, mapDatabaseError(err)
	}
	if control.DeviceState(deviceState) != control.DeviceActive {
		return control.TunnelConfiguration{}, false, control.ErrDeviceNotActive
	}

	keyHash := sha256.Sum256([]byte(idempotencyKey))
	requestHash := sha256.Sum256([]byte(regionID))
	operation := "create_tunnel_configuration:" + deviceID
	var storedRequestHash []byte
	var storedResponse []byte
	err = tx.QueryRow(queryContext, `
		SELECT request_hash, response_body
		FROM idempotency_keys
		WHERE user_id = $1::uuid AND operation = $2 AND key_hash = $3
		FOR UPDATE`, ownerID, operation, keyHash[:]).Scan(&storedRequestHash, &storedResponse)
	if err == nil {
		if !bytes.Equal(storedRequestHash, requestHash[:]) {
			return control.TunnelConfiguration{}, false, control.ErrConflict
		}
		var configuration control.TunnelConfiguration
		if err := json.Unmarshal(storedResponse, &configuration); err != nil {
			return control.TunnelConfiguration{}, false, fmt.Errorf("decode idempotent response: %w", err)
		}
		if err := tx.Commit(queryContext); err != nil {
			return control.TunnelConfiguration{}, false, fmt.Errorf("commit idempotent replay: %w", err)
		}
		return configuration, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return control.TunnelConfiguration{}, false, mapDatabaseError(err)
	}

	if _, err := tx.Exec(queryContext, `SELECT pg_advisory_xact_lock(hashtext($1))`, regionID); err != nil {
		return control.TunnelConfiguration{}, false, fmt.Errorf("lock region allocation: %w", err)
	}

	var ipv4Pool string
	var ipv6Pool string
	var dnsIPv4 string
	var dnsIPv6 string
	err = tx.QueryRow(queryContext, `
		SELECT ipv4_pool::text, ipv6_pool::text, host(dns_ipv4), host(dns_ipv6)
		FROM regions
		WHERE id = $1 AND available = true
		FOR UPDATE`, regionID).Scan(&ipv4Pool, &ipv6Pool, &dnsIPv4, &dnsIPv6)
	if err != nil {
		return control.TunnelConfiguration{}, false, mapDatabaseError(err)
	}

	// A device has at most one live lease. Reuse it when the caller retries
	// with a new idempotency key for the same region; otherwise the partial
	// unique index would turn a normal retry into a misleading 409 conflict.
	var existingAssignmentID string
	var existingIPv4 string
	var existingIPv6 string
	var existingRegionID string
	var existingEndpointHost string
	var existingEndpointPort int
	var existingServerPublicKey string
	var existingExpiresAt time.Time
	err = tx.QueryRow(queryContext, `
		SELECT pa.id::text, host(l.ipv4), host(l.ipv6), l.region_id,
		       g.endpoint_host, g.endpoint_port, g.public_key, pa.expires_at
		FROM peer_assignments pa
		JOIN ip_leases l ON l.id = pa.lease_id
		JOIN gateways g ON g.id = pa.gateway_id
		WHERE pa.device_id = $1::uuid
		  AND pa.state IN ('pending', 'active', 'removal_pending')
		  AND l.released_at IS NULL
		ORDER BY pa.created_at DESC
		LIMIT 1
		FOR UPDATE OF pa, l, g`, deviceID).Scan(
		&existingAssignmentID, &existingIPv4, &existingIPv6, &existingRegionID,
		&existingEndpointHost, &existingEndpointPort, &existingServerPublicKey, &existingExpiresAt)
	if err == nil {
		if existingRegionID != regionID {
			return control.TunnelConfiguration{}, false, control.ErrDeviceAlreadyAllocated
		}
		configuration := control.TunnelConfiguration{
			AssignmentID:               existingAssignmentID,
			Endpoint:                   net.JoinHostPort(existingEndpointHost, fmt.Sprintf("%d", existingEndpointPort)),
			ServerPublicKey:            existingServerPublicKey,
			Addresses:                  []string{existingIPv4 + "/32", existingIPv6 + "/128"},
			DNSServers:                 []string{dnsIPv4, dnsIPv6},
			AllowedIPs:                 []string{"0.0.0.0/0", "::/0"},
			PersistentKeepaliveSeconds: 25,
			ExpiresAt:                  existingExpiresAt,
		}
		responseBody, marshalErr := json.Marshal(configuration)
		if marshalErr != nil {
			return control.TunnelConfiguration{}, false, fmt.Errorf("encode reused tunnel configuration: %w", marshalErr)
		}
		_, insertErr := tx.Exec(queryContext, `
			INSERT INTO idempotency_keys (
			    user_id, operation, key_hash, request_hash, status_code,
			    response_body, expires_at
			) VALUES ($1::uuid, $2, $3, $4, 200, $5::jsonb, $6)`,
			ownerID, operation, keyHash[:], requestHash[:], responseBody, time.Now().UTC().Add(24*time.Hour))
		if insertErr != nil {
			return control.TunnelConfiguration{}, false, mapDatabaseError(insertErr)
		}
		if commitErr := tx.Commit(queryContext); commitErr != nil {
			return control.TunnelConfiguration{}, false, fmt.Errorf("commit reused tunnel configuration: %w", commitErr)
		}
		return configuration, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return control.TunnelConfiguration{}, false, mapDatabaseError(err)
	}

	var gatewayID string
	var endpointHost string
	var endpointPort int
	var serverPublicKey string
	err = tx.QueryRow(queryContext, `
		SELECT g.id::text, g.endpoint_host, g.endpoint_port, g.public_key
		FROM gateways g
		WHERE g.region_id = $1
		  AND g.state = 'ready'
		  AND (
		      SELECT count(*)
		      FROM peer_assignments pa
		      WHERE pa.gateway_id = g.id
		        AND pa.state IN ('pending', 'active', 'removal_pending')
		  ) < g.capacity_peers
		ORDER BY (
		    SELECT count(*)
		    FROM peer_assignments pa
		    WHERE pa.gateway_id = g.id
		      AND pa.state IN ('pending', 'active', 'removal_pending')
		), g.id
		LIMIT 1
		FOR UPDATE OF g SKIP LOCKED`, regionID).Scan(&gatewayID, &endpointHost, &endpointPort, &serverPublicKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return control.TunnelConfiguration{}, false, control.ErrCapacityExhausted
	}
	if err != nil {
		return control.TunnelConfiguration{}, false, mapDatabaseError(err)
	}

	usedIPv4, usedIPv6, err := activeAddresses(queryContext, tx, regionID)
	if err != nil {
		return control.TunnelConfiguration{}, false, err
	}
	ipv4, err := firstAvailableAddress(ipv4Pool, usedIPv4)
	if err != nil {
		return control.TunnelConfiguration{}, false, err
	}
	ipv6, err := firstAvailableAddress(ipv6Pool, usedIPv6)
	if err != nil {
		return control.TunnelConfiguration{}, false, err
	}

	var leaseID string
	err = tx.QueryRow(queryContext, `
		INSERT INTO ip_leases (region_id, device_id, ipv4, ipv6)
		VALUES ($1, $2::uuid, $3::inet, $4::inet)
		RETURNING id::text`, regionID, deviceID, ipv4.String()+"/32", ipv6.String()+"/128").Scan(&leaseID)
	if err != nil {
		return control.TunnelConfiguration{}, false, mapDatabaseError(err)
	}

	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	var assignmentID string
	err = tx.QueryRow(queryContext, `
		INSERT INTO peer_assignments (device_id, gateway_id, lease_id, expires_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4)
		RETURNING id::text`, deviceID, gatewayID, leaseID, expiresAt).Scan(&assignmentID)
	if err != nil {
		return control.TunnelConfiguration{}, false, mapDatabaseError(err)
	}

	configuration := control.TunnelConfiguration{
		AssignmentID:               assignmentID,
		Endpoint:                   net.JoinHostPort(endpointHost, fmt.Sprintf("%d", endpointPort)),
		ServerPublicKey:            serverPublicKey,
		Addresses:                  []string{ipv4.String() + "/32", ipv6.String() + "/128"},
		DNSServers:                 []string{dnsIPv4, dnsIPv6},
		AllowedIPs:                 []string{"0.0.0.0/0", "::/0"},
		PersistentKeepaliveSeconds: 25,
		ExpiresAt:                  expiresAt,
	}
	responseBody, err := json.Marshal(configuration)
	if err != nil {
		return control.TunnelConfiguration{}, false, fmt.Errorf("encode idempotent response: %w", err)
	}
	_, err = tx.Exec(queryContext, `
		INSERT INTO idempotency_keys (
		    user_id, operation, key_hash, request_hash, status_code,
		    response_body, expires_at
		) VALUES ($1::uuid, $2, $3, $4, 201, $5::jsonb, $6)`,
		ownerID, operation, keyHash[:], requestHash[:], responseBody, time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		return control.TunnelConfiguration{}, false, mapDatabaseError(err)
	}

	if err := tx.Commit(queryContext); err != nil {
		return control.TunnelConfiguration{}, false, fmt.Errorf("commit tunnel allocation: %w", err)
	}
	return configuration, true, nil
}

func activeAddresses(ctx context.Context, tx pgx.Tx, regionID string) (map[netip.Addr]struct{}, map[netip.Addr]struct{}, error) {
	rows, err := tx.Query(ctx, `
		SELECT host(ipv4), host(ipv6)
		FROM ip_leases
		WHERE region_id = $1 AND released_at IS NULL`, regionID)
	if err != nil {
		return nil, nil, fmt.Errorf("list active addresses: %w", err)
	}
	defer rows.Close()
	usedIPv4 := make(map[netip.Addr]struct{})
	usedIPv6 := make(map[netip.Addr]struct{})
	for rows.Next() {
		var ipv4Text string
		var ipv6Text string
		if err := rows.Scan(&ipv4Text, &ipv6Text); err != nil {
			return nil, nil, fmt.Errorf("scan active addresses: %w", err)
		}
		ipv4, err := netip.ParseAddr(ipv4Text)
		if err != nil {
			return nil, nil, fmt.Errorf("parse stored IPv4: %w", err)
		}
		ipv6, err := netip.ParseAddr(ipv6Text)
		if err != nil {
			return nil, nil, fmt.Errorf("parse stored IPv6: %w", err)
		}
		usedIPv4[ipv4] = struct{}{}
		usedIPv6[ipv6] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate active addresses: %w", err)
	}
	return usedIPv4, usedIPv6, nil
}

func firstAvailableAddress(poolText string, used map[netip.Addr]struct{}) (netip.Addr, error) {
	pool, err := netip.ParsePrefix(poolText)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("parse address pool: %w", err)
	}
	// Reserve the first address for the regional gateway/DNS service.
	candidate := pool.Masked().Addr().Next().Next()
	for attempts := 0; attempts < 65535 && candidate.IsValid() && pool.Contains(candidate); attempts++ {
		// Keep the final IPv4 address reserved instead of treating it as a host.
		if candidate.Is4() && !pool.Contains(candidate.Next()) {
			break
		}
		if _, exists := used[candidate]; !exists {
			return candidate, nil
		}
		candidate = candidate.Next()
	}
	return netip.Addr{}, control.ErrCapacityExhausted
}

func requireTLSForRemoteHost(config *pgxpool.Config) error {
	host := config.ConnConfig.Host
	if host == "" || strings.EqualFold(host, "localhost") {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	if config.ConnConfig.TLSConfig == nil {
		return errors.New("remote PostgreSQL connections require TLS")
	}
	return nil
}

func mapDatabaseError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return control.ErrNotFound
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23505":
			return control.ErrConflict
		case "23503":
			return control.ErrNotFound
		case "22P02", "23514":
			return control.ErrInvalidInput
		}
	}
	return fmt.Errorf("database operation: %w", err)
}
