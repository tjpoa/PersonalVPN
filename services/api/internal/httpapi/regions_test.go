package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"personalvpn/services/api/internal/control"
)

type fakeRegionService struct{}

func (fakeRegionService) ListRegions(context.Context) ([]control.Region, error) {
	return []control.Region{{ID: "pt-test", DisplayName: "Portugal", Available: true, Endpoint: "private.invalid", ServerPublicKey: "secret"}}, nil
}

func TestRegionListRequiresBearerToken(t *testing.T) {
	handler := NewRegionHandler(&fakeAuthService{accessToken: "valid-token", userID: "user-1"}, fakeRegionService{})
	request := httptest.NewRequest(http.MethodGet, "/v1/regions", nil)
	response := httptest.NewRecorder()
	handler.List(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestRegionListReturnsOnlyPublicMetadata(t *testing.T) {
	handler := NewRegionHandler(&fakeAuthService{accessToken: "valid-token", userID: "user-1"}, fakeRegionService{})
	request := httptest.NewRequest(http.MethodGet, "/v1/regions", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	handler.List(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	if body == "" || containsAny(body, "private.invalid", "secret", "endpoint") {
		t.Fatalf("response leaked private region fields: %s", body)
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
