package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"personalvpn/services/api/internal/control"
)

type DeviceService interface {
	ListDevices(ctx context.Context, ownerID string) ([]control.Device, error)
	RegisterDevice(ctx context.Context, ownerID string, request control.RegisterDeviceRequest) (control.Device, error)
	RequestRevocation(ctx context.Context, ownerID, deviceID string) error
	CreateTunnelConfiguration(ctx context.Context, ownerID, deviceID, regionID, idempotencyKey string) (control.TunnelConfiguration, bool, error)
}

type DeviceHandler struct {
	auth    AuthService
	devices DeviceService
}

func NewDeviceHandler(authService AuthService, devices DeviceService) *DeviceHandler {
	return &DeviceHandler{auth: authService, devices: devices}
}

func (h *DeviceHandler) List(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := h.authenticatedUser(w, r)
	if !ok {
		return
	}
	devices, err := h.devices.ListDevices(r.Context(), ownerID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "could not list devices")
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

func (h *DeviceHandler) Register(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := h.authenticatedUser(w, r)
	if !ok {
		return
	}
	var request struct {
		Name      string           `json:"name"`
		Platform  control.Platform `json:"platform"`
		PublicKey string           `json:"publicKey"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	device, err := h.devices.RegisterDevice(r.Context(), ownerID, control.RegisterDeviceRequest{
		Name: request.Name, Platform: request.Platform, PublicKey: request.PublicKey,
	})
	if err != nil {
		switch {
		case errors.Is(err, control.ErrConflict):
			writeProblem(w, http.StatusConflict, "device key already registered")
		case errors.Is(err, control.ErrInvalidInput):
			writeProblem(w, http.StatusBadRequest, "invalid device")
		default:
			writeProblem(w, http.StatusInternalServerError, "could not register device")
		}
		return
	}
	writeJSON(w, http.StatusCreated, device)
}

func (h *DeviceHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := h.authenticatedUser(w, r)
	if !ok {
		return
	}
	if err := h.devices.RequestRevocation(r.Context(), ownerID, r.PathValue("deviceId")); err != nil {
		if errors.Is(err, control.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "device not found")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "could not revoke device")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DeviceHandler) Configuration(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := h.authenticatedUser(w, r)
	if !ok {
		return
	}
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
		writeProblem(w, http.StatusBadRequest, "valid Idempotency-Key header is required")
		return
	}
	var request struct {
		RegionID string `json:"regionId"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	configuration, created, err := h.devices.CreateTunnelConfiguration(
		r.Context(), ownerID, r.PathValue("deviceId"), request.RegionID, idempotencyKey,
	)
	if err != nil {
		switch {
		case errors.Is(err, control.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "device or region not found")
		case errors.Is(err, control.ErrDeviceNotActive):
			writeProblem(w, http.StatusConflict, "device is not active")
		case errors.Is(err, control.ErrDeviceAlreadyAllocated):
			writeProblem(w, http.StatusConflict, "device already has an active tunnel in another region")
		case errors.Is(err, control.ErrCapacityExhausted):
			writeProblem(w, http.StatusServiceUnavailable, "region capacity is unavailable")
		case errors.Is(err, control.ErrConflict):
			writeProblem(w, http.StatusConflict, "idempotency key was reused with different data")
		case errors.Is(err, control.ErrInvalidInput):
			writeProblem(w, http.StatusBadRequest, "invalid tunnel request")
		default:
			writeProblem(w, http.StatusInternalServerError, "could not create tunnel configuration")
		}
		return
	}
	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	writeJSON(w, status, configuration)
}

func (h *DeviceHandler) authenticatedUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "missing bearer token")
		return "", false
	}
	userID, err := h.auth.AuthenticateAccess(token)
	if err != nil {
		writeProblem(w, http.StatusUnauthorized, "invalid access token")
		return "", false
	}
	return userID, true
}
