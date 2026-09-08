package control

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func testKey(seed byte) string {
	value := make([]byte, 32)
	for index := range value {
		value[index] = seed
	}
	return base64.StdEncoding.EncodeToString(value)
}

func testService(t *testing.T) *Service {
	t.Helper()
	service, err := NewService([]Region{{
		ID:              "pt-lis",
		DisplayName:     "Portugal — Lisboa",
		Available:       true,
		Endpoint:        "pt-lis.example.invalid:51820",
		ServerPublicKey: testKey(99),
		DNSv4:           "10.70.0.1",
		DNSv6:           "fd70::1",
	}})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	service.now = func() time.Time { return time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC) }
	return service
}

func registerTestDevice(t *testing.T, service *Service, owner string, seed byte) Device {
	t.Helper()
	device, err := service.RegisterDevice(owner, RegisterDeviceRequest{
		Name:      "Test device",
		Platform:  PlatformAndroid,
		PublicKey: testKey(seed),
	})
	if err != nil {
		t.Fatalf("RegisterDevice() error = %v", err)
	}
	return device
}

func TestRegisterDeviceRejectsInvalidInputs(t *testing.T) {
	service := testService(t)
	tests := []RegisterDeviceRequest{
		{Name: "", Platform: PlatformAndroid, PublicKey: testKey(1)},
		{Name: "phone", Platform: "other", PublicKey: testKey(1)},
		{Name: "phone", Platform: PlatformAndroid, PublicKey: "not-a-key"},
	}
	for _, request := range tests {
		if _, err := service.RegisterDevice("owner-a", request); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("RegisterDevice(%+v) error = %v, want ErrInvalidInput", request, err)
		}
	}
}

func TestPublicKeyCannotBeRegisteredTwice(t *testing.T) {
	service := testService(t)
	registerTestDevice(t, service, "owner-a", 1)
	_, err := service.RegisterDevice("owner-b", RegisterDeviceRequest{
		Name: "second", Platform: PlatformIOS, PublicKey: testKey(1),
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("RegisterDevice() error = %v, want ErrConflict", err)
	}
}

func TestListDevicesIsOwnerScoped(t *testing.T) {
	service := testService(t)
	registerTestDevice(t, service, "owner-a", 1)
	registerTestDevice(t, service, "owner-b", 2)
	devices := service.ListDevices("owner-a")
	if len(devices) != 1 || devices[0].ownerID != "owner-a" {
		t.Fatalf("ListDevices() = %+v, want only owner-a", devices)
	}
}

func TestConfigurationIsIdempotent(t *testing.T) {
	service := testService(t)
	device := registerTestDevice(t, service, "owner-a", 1)
	first, created, err := service.CreateTunnelConfiguration("owner-a", device.ID, "pt-lis", "0123456789abcdef")
	if err != nil || !created {
		t.Fatalf("first CreateTunnelConfiguration() = (%+v, %v, %v)", first, created, err)
	}
	second, created, err := service.CreateTunnelConfiguration("owner-a", device.ID, "pt-lis", "0123456789abcdef")
	if err != nil || created {
		t.Fatalf("second CreateTunnelConfiguration() = (%+v, %v, %v)", second, created, err)
	}
	if first.AssignmentID != second.AssignmentID || first.Addresses[0] != second.Addresses[0] {
		t.Fatalf("idempotent response changed: first=%+v second=%+v", first, second)
	}
	if first.ServerPublicKey == "" || len(first.Addresses) != 2 || len(first.AllowedIPs) != 2 {
		t.Fatalf("configuration incomplete: %+v", first)
	}
}

func TestConfigurationHidesOtherOwnersDevice(t *testing.T) {
	service := testService(t)
	device := registerTestDevice(t, service, "owner-a", 1)
	_, _, err := service.CreateTunnelConfiguration("owner-b", device.ID, "pt-lis", "0123456789abcdef")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("CreateTunnelConfiguration() error = %v, want ErrNotFound", err)
	}
}

func TestRevocationPreventsNewConfiguration(t *testing.T) {
	service := testService(t)
	device := registerTestDevice(t, service, "owner-a", 1)
	if err := service.RequestRevocation("owner-a", device.ID); err != nil {
		t.Fatalf("RequestRevocation() error = %v", err)
	}
	_, _, err := service.CreateTunnelConfiguration("owner-a", device.ID, "pt-lis", "0123456789abcdef")
	if !errors.Is(err, ErrDeviceNotActive) {
		t.Fatalf("CreateTunnelConfiguration() error = %v, want ErrDeviceNotActive", err)
	}
}

func TestRevocationHidesOtherOwnersDevice(t *testing.T) {
	service := testService(t)
	device := registerTestDevice(t, service, "owner-a", 1)
	if err := service.RequestRevocation("owner-b", device.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RequestRevocation() error = %v, want ErrNotFound", err)
	}
}
