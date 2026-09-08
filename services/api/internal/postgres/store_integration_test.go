//go:build integration

package postgres

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"personalvpn/services/api/internal/agent"
	"personalvpn/services/api/internal/control"
)

func integrationKey(seed byte) string {
	value := make([]byte, 32)
	for index := range value {
		value[index] = seed
	}
	return base64.StdEncoding.EncodeToString(value)
}

func TestDevicePersistence(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	store, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	suffix := time.Now().UTC().UnixNano()
	ownerA, err := store.CreateUser(ctx, fmt.Sprintf("owner-a-%d@example.invalid", suffix), "argon2id-test-placeholder")
	if err != nil {
		t.Fatalf("CreateUser(ownerA) error = %v", err)
	}
	ownerB, err := store.CreateUser(ctx, fmt.Sprintf("owner-b-%d@example.invalid", suffix), "argon2id-test-placeholder")
	if err != nil {
		t.Fatalf("CreateUser(ownerB) error = %v", err)
	}
	request := control.RegisterDeviceRequest{Name: "Phone", Platform: control.PlatformAndroid, PublicKey: integrationKey(21)}
	device, err := store.RegisterDevice(ctx, ownerA, request)
	if err != nil {
		t.Fatalf("RegisterDevice() error = %v", err)
	}
	if device.ID == "" || device.State != control.DeviceActive || device.Platform != control.PlatformAndroid {
		t.Fatalf("RegisterDevice() returned incomplete device: %+v", device)
	}

	if _, err := store.RegisterDevice(ctx, ownerB, request); !errors.Is(err, control.ErrConflict) {
		t.Fatalf("duplicate RegisterDevice() error = %v, want ErrConflict", err)
	}
	ownerADevices, err := store.ListDevices(ctx, ownerA)
	if err != nil || len(ownerADevices) != 1 || ownerADevices[0].ID != device.ID {
		t.Fatalf("ListDevices(ownerA) = (%+v, %v)", ownerADevices, err)
	}
	ownerBDevices, err := store.ListDevices(ctx, ownerB)
	if err != nil || len(ownerBDevices) != 0 {
		t.Fatalf("ListDevices(ownerB) = (%+v, %v), want empty", ownerBDevices, err)
	}

	if err := store.RequestRevocation(ctx, ownerB, device.ID); !errors.Is(err, control.ErrNotFound) {
		t.Fatalf("cross-owner RequestRevocation() error = %v, want ErrNotFound", err)
	}
	if err := store.RequestRevocation(ctx, ownerA, device.ID); err != nil {
		t.Fatalf("RequestRevocation() error = %v", err)
	}
	ownerADevices, err = store.ListDevices(ctx, ownerA)
	if err != nil || ownerADevices[0].State != control.DeviceRevocationPending {
		t.Fatalf("revoked ListDevices(ownerA) = (%+v, %v)", ownerADevices, err)
	}
}

func TestTunnelAllocationIsAtomicAndIdempotent(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	store, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	suffix := time.Now().UTC().UnixNano()
	ownerA, err := store.CreateUser(ctx, fmt.Sprintf("allocation-a-%d@example.invalid", suffix), "argon2id-test-placeholder")
	if err != nil {
		t.Fatalf("CreateUser(ownerA) error = %v", err)
	}
	ownerB, err := store.CreateUser(ctx, fmt.Sprintf("allocation-b-%d@example.invalid", suffix), "argon2id-test-placeholder")
	if err != nil {
		t.Fatalf("CreateUser(ownerB) error = %v", err)
	}
	_, err = store.pool.Exec(ctx, `
		INSERT INTO gateways (
		    region_id, name, endpoint_host, endpoint_port, public_key,
		    state, capacity_peers
		) VALUES (
		    'pt-test', $1, 'allocation.example.invalid', 51820, $2,
		    'ready', 1
		)`, fmt.Sprintf("allocation-gateway-%d", suffix), integrationKey(89))
	if err != nil {
		t.Fatalf("insert allocation gateway error = %v", err)
	}
	deviceA, err := store.RegisterDevice(ctx, ownerA, control.RegisterDeviceRequest{
		Name: "Device A", Platform: control.PlatformWindows, PublicKey: integrationKey(31),
	})
	if err != nil {
		t.Fatalf("RegisterDevice(A) error = %v", err)
	}
	deviceB, err := store.RegisterDevice(ctx, ownerB, control.RegisterDeviceRequest{
		Name: "Device B", Platform: control.PlatformIOS, PublicKey: integrationKey(32),
	})
	if err != nil {
		t.Fatalf("RegisterDevice(B) error = %v", err)
	}

	const idempotencyKey = "allocation-test-key-0001"
	configuration, created, err := store.CreateTunnelConfiguration(ctx, ownerA, deviceA.ID, "pt-test", idempotencyKey)
	if err != nil || !created {
		t.Fatalf("first CreateTunnelConfiguration() = (%+v, %v, %v)", configuration, created, err)
	}
	if len(configuration.Addresses) != 2 || configuration.Endpoint != "allocation.example.invalid:51820" {
		t.Fatalf("configuration incomplete: %+v", configuration)
	}
	replayed, created, err := store.CreateTunnelConfiguration(ctx, ownerA, deviceA.ID, "pt-test", idempotencyKey)
	if err != nil || created || replayed.AssignmentID != configuration.AssignmentID {
		t.Fatalf("replayed CreateTunnelConfiguration() = (%+v, %v, %v)", replayed, created, err)
	}
	// A client may retry after losing the response with a new idempotency key.
	// The existing live lease must be reused instead of allocating a second one.
	retried, created, err := store.CreateTunnelConfiguration(ctx, ownerA, deviceA.ID, "pt-test", "allocation-test-key-0002")
	if err != nil || !created || retried.AssignmentID != configuration.AssignmentID {
		t.Fatalf("new-key retry CreateTunnelConfiguration() = (%+v, %v, %v)", retried, created, err)
	}
	_, _, err = store.CreateTunnelConfiguration(ctx, ownerA, deviceA.ID, "changed-region", idempotencyKey)
	if !errors.Is(err, control.ErrConflict) {
		t.Fatalf("changed replay error = %v, want ErrConflict", err)
	}
	_, _, err = store.CreateTunnelConfiguration(ctx, ownerB, deviceA.ID, "pt-test", "cross-owner-test-key")
	if !errors.Is(err, control.ErrNotFound) {
		t.Fatalf("cross-owner allocation error = %v, want ErrNotFound", err)
	}
	_, _, err = store.CreateTunnelConfiguration(ctx, ownerB, deviceB.ID, "pt-test", "capacity-test-key-01")
	if !errors.Is(err, control.ErrCapacityExhausted) {
		t.Fatalf("capacity allocation error = %v, want ErrCapacityExhausted", err)
	}
}

func TestRemoteDatabaseWithoutTLSIsRejected(t *testing.T) {
	_, err := Open(context.Background(), "postgres://user:pass@192.0.2.1/database?sslmode=disable")
	if err == nil || err.Error() != "remote PostgreSQL connections require TLS" {
		t.Fatalf("Open(remote sslmode=disable) error = %v", err)
	}
}

func TestSnapshotOutboxClaimAckAndRetry(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	store, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	suffix := time.Now().UTC().UnixNano()
	var gatewayID string
	err = store.pool.QueryRow(ctx, `
		INSERT INTO gateways (
		    region_id, name, endpoint_host, endpoint_port, public_key,
		    state, capacity_peers
		) VALUES (
		    'pt-test', $1, 'outbox.example.invalid', 51820, $2,
		    'ready', 10
		) RETURNING id::text`, fmt.Sprintf("outbox-gateway-%d", suffix), integrationKey(byte(suffix%200+40))).Scan(&gatewayID)
	if err != nil {
		t.Fatalf("insert outbox gateway error = %v", err)
	}

	payload := []byte(`{"gatewayId":"gateway-1","generation":1,"peers":[]}`)
	signature := make([]byte, 64)
	if err := store.EnqueueSnapshot(ctx, gatewayID, 1, payload, signature); err != nil {
		t.Fatalf("EnqueueSnapshot() error = %v", err)
	}
	if err := store.EnqueueSnapshot(ctx, gatewayID, 1, payload, signature); !errors.Is(err, control.ErrConflict) {
		t.Fatalf("duplicate EnqueueSnapshot() error = %v, want ErrConflict", err)
	}
	delivery, err := store.ClaimNextSnapshot(ctx, gatewayID)
	if err != nil {
		t.Fatalf("ClaimNextSnapshot() error = %v", err)
	}
	var decodedPayload map[string]any
	if err := json.Unmarshal(delivery.Payload, &decodedPayload); err != nil {
		t.Fatalf("delivery payload is invalid JSON: %v", err)
	}
	if delivery.Generation != 1 || delivery.Attempts != 1 || decodedPayload["gatewayId"] != "gateway-1" {
		t.Fatalf("delivery = %+v, want generation 1 attempt 1", delivery)
	}
	if err := store.AckSnapshot(ctx, gatewayID, delivery.ID, 1); err != nil {
		t.Fatalf("AckSnapshot() error = %v", err)
	}
	if _, err := store.ClaimNextSnapshot(ctx, gatewayID); !errors.Is(err, control.ErrNotFound) {
		t.Fatalf("ClaimNextSnapshot(after ack) error = %v, want ErrNotFound", err)
	}

	if err := store.EnqueueSnapshot(ctx, gatewayID, 2, payload, signature); err != nil {
		t.Fatalf("EnqueueSnapshot(2) error = %v", err)
	}
	delivery, err = store.ClaimNextSnapshot(ctx, gatewayID)
	if err != nil {
		t.Fatalf("ClaimNextSnapshot(2) error = %v", err)
	}
	if err := store.RetrySnapshot(ctx, gatewayID, delivery.ID, time.Now().UTC().Add(-time.Second), "temporary agent failure"); err != nil {
		t.Fatalf("RetrySnapshot() error = %v", err)
	}
	delivery, err = store.ClaimNextSnapshot(ctx, gatewayID)
	if err != nil || delivery.Attempts != 2 {
		t.Fatalf("ClaimNextSnapshot(retry) = (%+v, %v), want attempt 2", delivery, err)
	}
	nextGeneration, err := store.NextGatewaySnapshotGeneration(ctx, gatewayID)
	if err != nil || nextGeneration != 3 {
		t.Fatalf("NextGatewaySnapshotGeneration() = (%d, %v), want 3", nextGeneration, err)
	}
}

func TestEnqueueCurrentGatewaySnapshotSignsDelivery(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	store, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	suffix := time.Now().UTC().UnixNano()
	var gatewayID string
	err = store.pool.QueryRow(ctx, `INSERT INTO gateways (region_id, name, endpoint_host, endpoint_port, public_key, state, capacity_peers) VALUES ('pt-test', $1, 'signed.example.invalid', 51820, $2, 'ready', 10) RETURNING id::text`, fmt.Sprintf("signed-gateway-%d", suffix), integrationKey(byte(suffix%200+40))).Scan(&gatewayID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueCurrentGatewaySnapshot(ctx, gatewayID, 1, privateKey); err != nil {
		t.Fatal(err)
	}
	delivery, err := store.ClaimNextSnapshot(ctx, gatewayID)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot agent.Snapshot
	if err := json.Unmarshal(delivery.Payload, &snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.Signature = delivery.Signature
	if err := snapshot.Verify(time.Now().UTC(), gatewayID, publicKey); err != nil {
		t.Fatalf("snapshot signature verification failed: %v", err)
	}
}

func TestGatewaysNeedingSnapshotSkipsInFlightDelivery(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	store, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	suffix := time.Now().UTC().UnixNano()
	var gatewayID, userID, deviceID, leaseID string
	if err := store.pool.QueryRow(ctx, `INSERT INTO gateways (region_id, name, endpoint_host, public_key, state, capacity_peers) VALUES ('pt-test', $1, 'reconcile.example.invalid', $2, 'ready', 10) RETURNING id::text`, fmt.Sprintf("reconcile-gateway-%d", suffix), integrationKey(byte(suffix%200+40))).Scan(&gatewayID); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `INSERT INTO users (email_normalized, password_hash) VALUES ($1, 'hash') RETURNING id::text`, fmt.Sprintf("reconcile-%d@example.invalid", suffix)).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `INSERT INTO devices (user_id, name, platform, public_key) VALUES ($1::uuid, 'reconcile-device', 'android', $2) RETURNING id::text`, userID, integrationKey(byte(suffix%200+41))).Scan(&deviceID); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `INSERT INTO ip_leases (region_id, device_id, ipv4, ipv6) VALUES ('pt-test', $1::uuid, $2::inet, $3::inet) RETURNING id::text`, deviceID, fmt.Sprintf("10.88.%d.2/32", suffix%200), fmt.Sprintf("fd88::%x/128", suffix%65535)).Scan(&leaseID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, `INSERT INTO peer_assignments (device_id, gateway_id, lease_id, state, desired_generation, expires_at) VALUES ($1::uuid, $2::uuid, $3::uuid, 'pending', 1, now() + interval '1 hour')`, deviceID, gatewayID, leaseID); err != nil {
		t.Fatal(err)
	}
	ids, err := store.GatewaysNeedingSnapshot(ctx)
	found := false
	for _, id := range ids {
		if id == gatewayID {
			found = true
		}
	}
	if err != nil || !found {
		t.Fatalf("GatewaysNeedingSnapshot() = (%v, %v), want gateway %s", ids, err, gatewayID)
	}
	if err := store.EnqueueSnapshot(ctx, gatewayID, 1, []byte(`{}`), make([]byte, 64)); err != nil {
		t.Fatal(err)
	}
	ids, err = store.GatewaysNeedingSnapshot(ctx)
	found = false
	for _, id := range ids {
		if id == gatewayID {
			found = true
		}
	}
	if err != nil || found {
		t.Fatalf("GatewaysNeedingSnapshot(in-flight) = (%v, %v), want gateway absent", ids, err)
	}
}

func TestConfirmGatewaySnapshotFinalizesRevocation(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	store, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	suffix := time.Now().UTC().UnixNano()
	var userID, deviceID, gatewayID, leaseID string
	if err := store.pool.QueryRow(ctx, `INSERT INTO users (email_normalized, password_hash) VALUES ($1, 'test-hash') RETURNING id::text`, fmt.Sprintf("revoke-%d@example.invalid", suffix)).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `INSERT INTO devices (user_id, name, platform, public_key, state) VALUES ($1::uuid, 'revoke-device', 'android', $2, 'revocation_pending') RETURNING id::text`, userID, integrationKey(byte(suffix%200+40))).Scan(&deviceID); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `INSERT INTO gateways (region_id, name, endpoint_host, endpoint_port, public_key, state, capacity_peers) VALUES ('pt-test', $1, 'revoke.example.invalid', 51820, $2, 'ready', 10) RETURNING id::text`, fmt.Sprintf("revoke-gateway-%d", suffix), integrationKey(byte(suffix%200+80))).Scan(&gatewayID); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `INSERT INTO ip_leases (region_id, device_id, ipv4, ipv6) VALUES ('pt-test', $1::uuid, '10.77.0.2/32', 'fd77::2/128') RETURNING id::text`, deviceID).Scan(&leaseID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, `INSERT INTO peer_assignments (device_id, gateway_id, lease_id, state, desired_generation, expires_at) VALUES ($1::uuid, $2::uuid, $3::uuid, 'removal_pending', 1, now() + interval '1 hour')`, deviceID, gatewayID, leaseID); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmGatewaySnapshot(ctx, gatewayID, 1); err != nil {
		t.Fatal(err)
	}
	var assignmentState, deviceState string
	var releasedAt *time.Time
	if err := store.pool.QueryRow(ctx, `SELECT pa.state::text, d.state::text, l.released_at FROM peer_assignments pa JOIN devices d ON d.id = pa.device_id JOIN ip_leases l ON l.id = pa.lease_id WHERE pa.gateway_id = $1::uuid`, gatewayID).Scan(&assignmentState, &deviceState, &releasedAt); err != nil {
		t.Fatal(err)
	}
	if assignmentState != "removed" || deviceState != "revoked" || releasedAt == nil {
		t.Fatalf("states = %s/%s released=%v", assignmentState, deviceState, releasedAt)
	}
}
