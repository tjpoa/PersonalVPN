package control

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidInput           = errors.New("invalid input")
	ErrNotFound               = errors.New("resource not found")
	ErrConflict               = errors.New("resource conflict")
	ErrCapacityExhausted      = errors.New("region capacity exhausted")
	ErrDeviceNotActive        = errors.New("device is not active")
	ErrDeviceAlreadyAllocated = errors.New("device already has an active tunnel")
)

type Platform string

const (
	PlatformWindows   Platform = "windows"
	PlatformAndroid   Platform = "android"
	PlatformIOS       Platform = "ios"
	PlatformAndroidTV Platform = "android_tv"
)

type DeviceState string

const (
	DeviceActive            DeviceState = "active"
	DeviceRevocationPending DeviceState = "revocation_pending"
	DeviceRevoked           DeviceState = "revoked"
)

type Device struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Platform  Platform    `json:"platform"`
	PublicKey string      `json:"publicKey"`
	State     DeviceState `json:"state"`
	CreatedAt time.Time   `json:"createdAt"`

	ownerID string
}

type Region struct {
	ID              string `json:"id"`
	DisplayName     string `json:"displayName"`
	Available       bool   `json:"available"`
	Endpoint        string `json:"-"`
	ServerPublicKey string `json:"-"`
	DNSv4           string `json:"-"`
	DNSv6           string `json:"-"`
}

type RegisterDeviceRequest struct {
	Name      string
	Platform  Platform
	PublicKey string
}

type TunnelConfiguration struct {
	AssignmentID               string    `json:"assignmentId"`
	Endpoint                   string    `json:"endpoint"`
	ServerPublicKey            string    `json:"serverPublicKey"`
	Addresses                  []string  `json:"addresses"`
	DNSServers                 []string  `json:"dnsServers"`
	AllowedIPs                 []string  `json:"allowedIps"`
	PersistentKeepaliveSeconds int       `json:"persistentKeepaliveSeconds"`
	ExpiresAt                  time.Time `json:"expiresAt"`
}

type idempotencyRecord struct {
	regionID      string
	configuration TunnelConfiguration
}

type Service struct {
	mu sync.RWMutex

	devices        map[string]Device
	publicKeys     map[string]string
	regions        map[string]Region
	idempotency    map[string]idempotencyRecord
	regionNextHost map[string]uint16
	now            func() time.Time
}

func NewService(regions []Region) (*Service, error) {
	service := &Service{
		devices:        make(map[string]Device),
		publicKeys:     make(map[string]string),
		regions:        make(map[string]Region),
		idempotency:    make(map[string]idempotencyRecord),
		regionNextHost: make(map[string]uint16),
		now:            func() time.Time { return time.Now().UTC() },
	}

	for _, region := range regions {
		if err := validateRegion(region); err != nil {
			return nil, err
		}
		if _, exists := service.regions[region.ID]; exists {
			return nil, fmt.Errorf("%w: duplicate region", ErrConflict)
		}
		service.regions[region.ID] = region
		service.regionNextHost[region.ID] = 2
	}
	return service, nil
}

func (s *Service) ListRegions() []Region {
	s.mu.RLock()
	defer s.mu.RUnlock()

	regions := make([]Region, 0, len(s.regions))
	for _, region := range s.regions {
		regions = append(regions, region)
	}
	return regions
}

func (s *Service) RegisterDevice(ownerID string, request RegisterDeviceRequest) (Device, error) {
	if err := ValidateRegisterDevice(ownerID, request); err != nil {
		return Device{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.publicKeys[request.PublicKey]; exists {
		return Device{}, ErrConflict
	}

	id, err := randomUUID()
	if err != nil {
		return Device{}, fmt.Errorf("generate device id: %w", err)
	}
	device := Device{
		ID:        id,
		Name:      strings.TrimSpace(request.Name),
		Platform:  request.Platform,
		PublicKey: request.PublicKey,
		State:     DeviceActive,
		CreatedAt: s.now(),
		ownerID:   ownerID,
	}
	s.devices[id] = device
	s.publicKeys[request.PublicKey] = id
	return device, nil
}

func ValidateRegisterDevice(ownerID string, request RegisterDeviceRequest) error {
	if strings.TrimSpace(ownerID) == "" || strings.TrimSpace(request.Name) == "" || len(request.Name) > 80 {
		return ErrInvalidInput
	}
	if !validPlatform(request.Platform) || !validWireGuardPublicKey(request.PublicKey) {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) ListDevices(ownerID string) []Device {
	s.mu.RLock()
	defer s.mu.RUnlock()

	devices := make([]Device, 0)
	for _, device := range s.devices {
		if device.ownerID == ownerID {
			devices = append(devices, device)
		}
	}
	return devices
}

func (s *Service) RequestRevocation(ownerID, deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	device, exists := s.devices[deviceID]
	if !exists || device.ownerID != ownerID {
		return ErrNotFound
	}
	if device.State == DeviceRevoked {
		return nil
	}
	device.State = DeviceRevocationPending
	s.devices[deviceID] = device
	return nil
}

func (s *Service) CreateTunnelConfiguration(ownerID, deviceID, regionID, idempotencyKey string) (TunnelConfiguration, bool, error) {
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
		return TunnelConfiguration{}, false, ErrInvalidInput
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	device, exists := s.devices[deviceID]
	if !exists || device.ownerID != ownerID {
		return TunnelConfiguration{}, false, ErrNotFound
	}
	if device.State != DeviceActive {
		return TunnelConfiguration{}, false, ErrDeviceNotActive
	}
	region, exists := s.regions[regionID]
	if !exists || !region.Available {
		return TunnelConfiguration{}, false, ErrNotFound
	}

	recordKey := ownerID + "\x00" + deviceID + "\x00" + idempotencyKey
	if record, exists := s.idempotency[recordKey]; exists {
		if record.regionID != regionID {
			return TunnelConfiguration{}, false, ErrConflict
		}
		return cloneConfiguration(record.configuration), false, nil
	}

	host := s.regionNextHost[regionID]
	if host > 254 {
		return TunnelConfiguration{}, false, ErrCapacityExhausted
	}
	assignmentID, err := randomUUID()
	if err != nil {
		return TunnelConfiguration{}, false, fmt.Errorf("generate assignment id: %w", err)
	}
	configuration := TunnelConfiguration{
		AssignmentID:               assignmentID,
		Endpoint:                   region.Endpoint,
		ServerPublicKey:            region.ServerPublicKey,
		Addresses:                  []string{fmt.Sprintf("10.70.0.%d/32", host), fmt.Sprintf("fd70::%x/128", host)},
		DNSServers:                 []string{region.DNSv4, region.DNSv6},
		AllowedIPs:                 []string{"0.0.0.0/0", "::/0"},
		PersistentKeepaliveSeconds: 25,
		ExpiresAt:                  s.now().Add(24 * time.Hour),
	}
	s.regionNextHost[regionID]++
	s.idempotency[recordKey] = idempotencyRecord{regionID: regionID, configuration: cloneConfiguration(configuration)}
	return configuration, true, nil
}

func validPlatform(platform Platform) bool {
	switch platform {
	case PlatformWindows, PlatformAndroid, PlatformIOS, PlatformAndroidTV:
		return true
	default:
		return false
	}
}

func validWireGuardPublicKey(value string) bool {
	decoded, err := base64.StdEncoding.DecodeString(value)
	return err == nil && len(decoded) == 32 && base64.StdEncoding.EncodeToString(decoded) == value
}

func validateRegion(region Region) error {
	if strings.TrimSpace(region.ID) == "" || len(region.ID) > 32 || strings.TrimSpace(region.DisplayName) == "" {
		return ErrInvalidInput
	}
	if strings.TrimSpace(region.Endpoint) == "" || !validWireGuardPublicKey(region.ServerPublicKey) {
		return ErrInvalidInput
	}
	if strings.TrimSpace(region.DNSv4) == "" || strings.TrimSpace(region.DNSv6) == "" {
		return ErrInvalidInput
	}
	return nil
}

func randomUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}

func cloneConfiguration(configuration TunnelConfiguration) TunnelConfiguration {
	configuration.Addresses = append([]string(nil), configuration.Addresses...)
	configuration.DNSServers = append([]string(nil), configuration.DNSServers...)
	configuration.AllowedIPs = append([]string(nil), configuration.AllowedIPs...)
	return configuration
}
