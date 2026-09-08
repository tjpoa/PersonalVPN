package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewHTTPSourceRequiresHTTPS(t *testing.T) {
	if _, err := NewHTTPSource("http://control.example", "gw-1", nil); err != ErrInvalidTransport {
		t.Fatalf("error = %v", err)
	}
	if _, err := NewHTTPSource("https://control.example", "gw/1", nil); err != ErrInvalidTransport {
		t.Fatalf("error = %v", err)
	}
	for _, raw := range []string{"https://control.example?token=secret", "https://control.example/#fragment"} {
		if _, err := NewHTTPSource(raw, "gw-1", nil); err != ErrInvalidTransport {
			t.Fatalf("URL %q error = %v", raw, err)
		}
	}
}

func TestNewHTTPSourceDefaultClientRejectsRedirects(t *testing.T) {
	source, err := NewHTTPSource("https://control.example", "gw-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if source.client.CheckRedirect == nil {
		t.Fatal("default HTTP client must define a redirect policy")
	}
	if err := source.client.CheckRedirect(nil, nil); err != http.ErrUseLastResponse {
		t.Fatalf("redirect policy error = %v, want http.ErrUseLastResponse", err)
	}
}

func TestHTTPSourceClaimAckRetry(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/v1/agent/gateways/gw-1/") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/snapshots/next"):
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(Delivery{ID: "d1", Generation: 4, Payload: []byte(`{"gatewayId":"gw-1"}`), Signature: make([]byte, 64)})
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := server.Client()
	source, err := NewHTTPSource(strings.Replace(server.URL, "http://", "https://", 1), "gw-1", client)
	if err != nil {
		t.Fatal(err)
	}
	delivery, err := source.Claim(context.Background())
	if err != nil || delivery.Generation != 4 {
		t.Fatalf("Claim() = %+v, %v", delivery, err)
	}
	if err := source.Ack(context.Background(), "d1", 4); err != nil {
		t.Fatal(err)
	}
	if err := source.Retry(context.Background(), "d1", time.Now(), "temporary"); err != nil {
		t.Fatal(err)
	}
}
