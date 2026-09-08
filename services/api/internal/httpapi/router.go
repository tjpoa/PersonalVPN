package httpapi

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"time"
)

type HealthChecker interface {
	Ping(ctx context.Context) error
}

func NewMux(authService AuthService, deviceService DeviceService, healthChecker HealthChecker, gatewayServices ...GatewayDeliveryService) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /v1/ready", func(response http.ResponseWriter, request *http.Request) {
		if healthChecker == nil || healthChecker.Ping(request.Context()) != nil {
			writeProblem(response, http.StatusServiceUnavailable, "database is not ready")
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]string{"status": "ready"})
	})
	if authService != nil {
		handler := NewAuthHandler(authService)
		mux.HandleFunc("POST /v1/auth/register", handler.Register)
		mux.HandleFunc("POST /v1/auth/login", handler.Login)
		mux.HandleFunc("POST /v1/auth/refresh", handler.Refresh)
		mux.HandleFunc("POST /v1/auth/logout", handler.Logout)
	}
	if authService != nil && deviceService != nil {
		handler := NewDeviceHandler(authService, deviceService)
		mux.HandleFunc("GET /v1/devices", handler.List)
		mux.HandleFunc("POST /v1/devices", handler.Register)
		mux.HandleFunc("DELETE /v1/devices/{deviceId}", handler.Revoke)
		mux.HandleFunc("POST /v1/devices/{deviceId}/configuration", handler.Configuration)
	}
	if authService != nil {
		if regionService, ok := deviceService.(RegionService); ok {
			handler := NewRegionHandler(authService, regionService)
			mux.HandleFunc("GET /v1/regions", handler.List)
		}
	}
	if len(gatewayServices) > 0 && gatewayServices[0] != nil {
		handler := NewGatewayHandler(gatewayServices[0])
		mux.HandleFunc("GET /v1/agent/gateways/{gatewayId}/snapshots/next", handler.Claim)
		mux.HandleFunc("POST /v1/agent/gateways/{gatewayId}/snapshots/{deliveryId}/ack", handler.Ack)
		mux.HandleFunc("POST /v1/agent/gateways/{gatewayId}/snapshots/{deliveryId}/retry", handler.Retry)
	}
	return mux
}

func NewServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           securityHeaders(handler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 * 1024,
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func NewMTLSServer(address string, handler http.Handler, tlsConfig *tls.Config) *http.Server {
	server := NewServer(address, handler)
	server.TLSConfig = tlsConfig
	return server
}
