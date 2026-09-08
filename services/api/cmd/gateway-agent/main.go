package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"personalvpn/services/api/internal/agent"
)

func main() {
	gatewayID := strings.TrimSpace(os.Getenv("GATEWAY_ID"))
	controlURL := strings.TrimSpace(os.Getenv("CONTROL_API_URL"))
	interfaceName := strings.TrimSpace(os.Getenv("WG_INTERFACE"))
	verifyKey, err := parsePublicKey(os.Getenv("SNAPSHOT_VERIFY_KEY"))
	if err != nil || gatewayID == "" || controlURL == "" || interfaceName == "" {
		slog.Error("invalid gateway agent configuration")
		os.Exit(2)
	}
	interval := 5 * time.Second
	if raw := strings.TrimSpace(os.Getenv("AGENT_POLL_INTERVAL")); raw != "" {
		interval, err = time.ParseDuration(raw)
		if err != nil || interval < time.Second || interval > time.Minute {
			slog.Error("invalid AGENT_POLL_INTERVAL")
			os.Exit(2)
		}
	}
	tlsConfig, err := agent.ClientTLSConfig(agent.TLSFiles{
		CertificateFile: os.Getenv("AGENT_TLS_CERT_FILE"),
		PrivateKeyFile:  os.Getenv("AGENT_TLS_KEY_FILE"),
		CAFile:          os.Getenv("AGENT_TLS_CA_FILE"),
		ServerName:      os.Getenv("AGENT_TLS_SERVER_NAME"),
	})
	if err != nil {
		slog.Error("invalid agent TLS configuration")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	client := &http.Client{
		Timeout:       20 * time.Second,
		Transport:     &http.Transport{TLSClientConfig: tlsConfig},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	source, err := agent.NewHTTPSource(controlURL, gatewayID, client)
	if err != nil {
		slog.Error("invalid control API URL")
		os.Exit(2)
	}
	worker := &agent.Worker{
		Source:       source,
		Applier:      agent.WireGuardBackend{InterfaceName: interfaceName, Runner: agent.ProcessRunner{}},
		GatewayID:    gatewayID,
		VerifyKey:    verifyKey,
		CurrentPeers: make(map[string]agent.Peer),
	}
	if err := worker.Run(ctx, interval); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("gateway agent stopped")
		os.Exit(1)
	}
}

func parsePublicKey(value string) (ed25519.PublicKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(decoded) != ed25519.PublicKeySize {
		return nil, errors.New("expected base64 Ed25519 public key")
	}
	return ed25519.PublicKey(decoded), nil
}
