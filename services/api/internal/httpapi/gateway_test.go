package httpapi

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"personalvpn/services/api/internal/agent"
)

type fakeGatewayService struct {
	delivery       agent.Delivery
	acked, retried bool
}

func (s *fakeGatewayService) ClaimSnapshot(context.Context, string) (agent.Delivery, error) {
	return s.delivery, nil
}
func (s *fakeGatewayService) AckSnapshotDelivery(context.Context, string, string, int64) error {
	s.acked = true
	return nil
}
func (s *fakeGatewayService) RetrySnapshotDelivery(context.Context, string, string, time.Time, string) error {
	s.retried = true
	return nil
}

func gatewayRequest(method, target, commonName string, tlsEnabled bool) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	if tlsEnabled {
		r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{{Subject: pkix.Name{CommonName: commonName}}}}
	}
	return r
}

func TestGatewayClaimRequiresMatchingMTLSIdentity(t *testing.T) {
	service := &fakeGatewayService{delivery: agent.Delivery{ID: "d1", Generation: 1, Payload: []byte(`{}`), Signature: make([]byte, 64)}}
	handler := NewGatewayHandler(service)
	for _, test := range []struct {
		name, cn string
		tls      bool
		status   int
	}{
		{"missing TLS", "gw-1", false, http.StatusUnauthorized},
		{"wrong identity", "gw-2", true, http.StatusForbidden},
		{"accepted", "gw-1", true, http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := gatewayRequest(http.MethodGet, "/v1/agent/gateways/gw-1/snapshots/next", test.cn, test.tls)
			request.SetPathValue("gatewayId", "gw-1")
			response := httptest.NewRecorder()
			handler.Claim(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}
