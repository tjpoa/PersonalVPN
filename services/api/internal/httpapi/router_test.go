package httpapi

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeHealth struct{ err error }

func (f fakeHealth) Ping(context.Context) error { return f.err }

func TestMuxHealthAndReadiness(t *testing.T) {
	ready := NewMux(nil, nil, fakeHealth{})
	request := httptest.NewRequest(http.MethodGet, "/v1/ready", nil)
	response := httptest.NewRecorder()
	ready.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("ready mux status = %d, want 200", response.Code)
	}

	notReady := NewMux(nil, nil, fakeHealth{err: errors.New("database down")})
	response = httptest.NewRecorder()
	notReady.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("not-ready mux status = %d, want 503", response.Code)
	}

	health := httptest.NewRecorder()
	healthRequest := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	ready.ServeHTTP(health, healthRequest)
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", health.Code)
	}
}

func TestMuxDoesNotMountAuthWithoutService(t *testing.T) {
	server := NewMux(nil, nil, nil)
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("unconfigured auth route status = %d, want 404", response.Code)
	}
}

func TestServerAddsSecurityHeaders(t *testing.T) {
	server := NewServer(":0", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	request := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	request.TLS = &tls.ConnectionState{}
	response := httptest.NewRecorder()
	server.Handler.ServeHTTP(response, request)
	if response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("security headers missing: %v", response.Header())
	}
	if response.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("HSTS missing for TLS request")
	}
}
