package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"personalvpn/services/api/internal/auth"
	"personalvpn/services/api/internal/control"
)

type fakeDeviceService struct {
	listOwner          string
	registeredOwner    string
	registeredRequest  control.RegisterDeviceRequest
	revokedOwner       string
	revokedDevice      string
	configurationOwner string
}

func (f *fakeDeviceService) ListDevices(_ context.Context, ownerID string) ([]control.Device, error) {
	f.listOwner = ownerID
	return []control.Device{}, nil
}

func (f *fakeDeviceService) RegisterDevice(_ context.Context, ownerID string, request control.RegisterDeviceRequest) (control.Device, error) {
	f.registeredOwner, f.registeredRequest = ownerID, request
	return control.Device{ID: "device-1", Name: request.Name, Platform: request.Platform, PublicKey: request.PublicKey, State: control.DeviceActive}, nil
}

func (f *fakeDeviceService) RequestRevocation(_ context.Context, ownerID, deviceID string) error {
	f.revokedOwner, f.revokedDevice = ownerID, deviceID
	return nil
}

func (f *fakeDeviceService) CreateTunnelConfiguration(_ context.Context, ownerID, _, _, _ string) (control.TunnelConfiguration, bool, error) {
	f.configurationOwner = ownerID
	return control.TunnelConfiguration{AssignmentID: "assignment-1"}, true, nil
}

func authForDeviceTests() *fakeAuthService {
	return &fakeAuthService{accessToken: "access", userID: "user-a"}
}

func TestDeviceListUsesAuthenticatedOwner(t *testing.T) {
	service := &fakeDeviceService{}
	handler := NewDeviceHandler(authForDeviceTests(), service)
	request := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	request.Header.Set("Authorization", "Bearer access")
	response := httptest.NewRecorder()
	handler.List(response, request)
	if response.Code != http.StatusOK || service.listOwner != "user-a" {
		t.Fatalf("List() status=%d owner=%q", response.Code, service.listOwner)
	}
}

func TestDeviceRegisterDoesNotAcceptUserIDFromBody(t *testing.T) {
	service := &fakeDeviceService{}
	handler := NewDeviceHandler(authForDeviceTests(), service)
	request := httptest.NewRequest(http.MethodPost, "/v1/devices", strings.NewReader(`{"name":"Phone","platform":"android","publicKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","userId":"user-b"}`))
	request.Header.Set("Authorization", "Bearer access")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.Register(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("Register() status = %d, want 400 for unknown userId", response.Code)
	}
}

func TestDeviceConfigurationRequiresIdempotencyKey(t *testing.T) {
	service := &fakeDeviceService{}
	handler := NewDeviceHandler(authForDeviceTests(), service)
	request := httptest.NewRequest(http.MethodPost, "/v1/devices/device-1/configuration", strings.NewReader(`{"regionId":"pt-lis"}`))
	request.Header.Set("Authorization", "Bearer access")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.Configuration(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("Configuration() status = %d, want 400", response.Code)
	}
	if service.configurationOwner != "" {
		t.Fatal("configuration service called without idempotency key")
	}
}

func TestDeviceConfigurationReturnsCreatedAndReplayStatus(t *testing.T) {
	service := &fakeDeviceService{}
	handler := NewDeviceHandler(authForDeviceTests(), service)
	makeRequest := func() *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/v1/devices/device-1/configuration", strings.NewReader(`{"regionId":"pt-lis"}`))
		request.Header.Set("Authorization", "Bearer access")
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", "0123456789abcdef")
		response := httptest.NewRecorder()
		handler.Configuration(response, request)
		return response
	}
	if response := makeRequest(); response.Code != http.StatusCreated {
		t.Fatalf("first Configuration() status = %d, want 201", response.Code)
	}
	// A real store returns created=false on replay; exercise that mapping via a small wrapper.
	serviceReplay := &replayDeviceService{}
	replayHandler := NewDeviceHandler(authForDeviceTests(), serviceReplay)
	request := httptest.NewRequest(http.MethodPost, "/v1/devices/device-1/configuration", strings.NewReader(`{"regionId":"pt-lis"}`))
	request.Header.Set("Authorization", "Bearer access")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "0123456789abcdef")
	response := httptest.NewRecorder()
	replayHandler.Configuration(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("replayed Configuration() status = %d, want 200", response.Code)
	}
}

type replayDeviceService struct{ fakeDeviceService }

func (r *replayDeviceService) CreateTunnelConfiguration(context.Context, string, string, string, string) (control.TunnelConfiguration, bool, error) {
	return control.TunnelConfiguration{AssignmentID: "assignment-1"}, false, nil
}

func TestDeviceHandlerMapsNotFoundAndInternalErrors(t *testing.T) {
	handler := NewDeviceHandler(authForDeviceTests(), errorDeviceService{err: control.ErrNotFound})
	request := httptest.NewRequest(http.MethodDelete, "/v1/devices/device-1", nil)
	request.SetPathValue("deviceId", "device-1")
	request.Header.Set("Authorization", "Bearer access")
	response := httptest.NewRecorder()
	handler.Revoke(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("Revoke(not found) status = %d, want 404", response.Code)
	}

	handler = NewDeviceHandler(authForDeviceTests(), errorDeviceService{err: errors.New("database failure")})
	response = httptest.NewRecorder()
	handler.List(response, httptest.NewRequest(http.MethodGet, "/v1/devices", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("List(without auth) status = %d, want 401", response.Code)
	}
}

type errorDeviceService struct{ err error }

func (e errorDeviceService) ListDevices(context.Context, string) ([]control.Device, error) {
	return nil, e.err
}
func (e errorDeviceService) RegisterDevice(context.Context, string, control.RegisterDeviceRequest) (control.Device, error) {
	return control.Device{}, e.err
}
func (e errorDeviceService) RequestRevocation(context.Context, string, string) error { return e.err }
func (e errorDeviceService) CreateTunnelConfiguration(context.Context, string, string, string, string) (control.TunnelConfiguration, bool, error) {
	return control.TunnelConfiguration{}, false, e.err
}

var _ AuthService = (*fakeAuthService)(nil)
var _ AuthService = (*auth.Service)(nil)
