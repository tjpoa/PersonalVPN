package httpapi

import (
	"context"
	"net/http"

	"personalvpn/services/api/internal/control"
)

type RegionService interface {
	ListRegions(ctx context.Context) ([]control.Region, error)
}

type RegionHandler struct {
	auth    AuthService
	regions RegionService
}

func NewRegionHandler(auth AuthService, regions RegionService) *RegionHandler {
	return &RegionHandler{auth: auth, regions: regions}
}

func (h *RegionHandler) List(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok || h.auth == nil {
		writeProblem(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if _, err := h.auth.AuthenticateAccess(token); err != nil {
		writeProblem(w, http.StatusUnauthorized, "authentication required")
		return
	}
	regions, err := h.regions.ListRegions(r.Context())
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "regions unavailable")
		return
	}
	writeJSON(w, http.StatusOK, regions)
}
