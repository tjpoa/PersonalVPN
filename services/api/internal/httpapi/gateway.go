package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"personalvpn/services/api/internal/agent"
)

type GatewayDeliveryService interface {
	ClaimSnapshot(ctx context.Context, gatewayID string) (agent.Delivery, error)
	AckSnapshotDelivery(ctx context.Context, gatewayID, deliveryID string, generation int64) error
	RetrySnapshotDelivery(ctx context.Context, gatewayID, deliveryID string, retryAt time.Time, reason string) error
}

type GatewayHandler struct{ service GatewayDeliveryService }

func NewGatewayHandler(service GatewayDeliveryService) *GatewayHandler {
	return &GatewayHandler{service: service}
}

func (h *GatewayHandler) Claim(w http.ResponseWriter, r *http.Request) {
	gatewayID, ok := gatewayIdentity(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "gateway mTLS identity required")
		return
	}
	if gatewayID != r.PathValue("gatewayId") {
		writeProblem(w, http.StatusForbidden, "gateway identity mismatch")
		return
	}
	delivery, err := h.service.ClaimSnapshot(r.Context(), gatewayID)
	if errors.Is(err, agent.ErrNoDelivery) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "snapshot unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(delivery)
}

func (h *GatewayHandler) Ack(w http.ResponseWriter, r *http.Request) {
	gatewayID, ok := gatewayIdentity(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "gateway mTLS identity required")
		return
	}
	if gatewayID != r.PathValue("gatewayId") {
		writeProblem(w, http.StatusForbidden, "gateway identity mismatch")
		return
	}
	var request struct {
		Generation int64 `json:"generation"`
	}
	if !decodeJSON(w, r, &request) || request.Generation <= 0 {
		writeProblem(w, http.StatusBadRequest, "invalid generation")
		return
	}
	if err := h.service.AckSnapshotDelivery(r.Context(), gatewayID, r.PathValue("deliveryId"), request.Generation); err != nil {
		writeProblem(w, http.StatusNotFound, "delivery not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *GatewayHandler) Retry(w http.ResponseWriter, r *http.Request) {
	gatewayID, ok := gatewayIdentity(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "gateway mTLS identity required")
		return
	}
	if gatewayID != r.PathValue("gatewayId") {
		writeProblem(w, http.StatusForbidden, "gateway identity mismatch")
		return
	}
	var request struct {
		RetryAt time.Time `json:"retryAt"`
		Reason  string    `json:"reason"`
	}
	if !decodeJSON(w, r, &request) || request.RetryAt.IsZero() || len(request.Reason) > 512 {
		writeProblem(w, http.StatusBadRequest, "invalid retry request")
		return
	}
	if err := h.service.RetrySnapshotDelivery(r.Context(), gatewayID, r.PathValue("deliveryId"), request.RetryAt, request.Reason); err != nil {
		writeProblem(w, http.StatusNotFound, "delivery not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func gatewayIdentity(r *http.Request) (string, bool) {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return "", false
	}
	certificate := r.TLS.PeerCertificates[0]
	if certificate.Subject.CommonName == "" || strings.ContainsAny(certificate.Subject.CommonName, "/?#") {
		return "", false
	}
	return certificate.Subject.CommonName, true
}
