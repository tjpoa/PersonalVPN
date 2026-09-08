package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"personalvpn/services/api/internal/postgres"
)

func main() {
	keyFile := os.Getenv("SNAPSHOT_SIGNING_KEY_FILE")
	if keyFile == "" {
		slog.Error("SNAPSHOT_SIGNING_KEY_FILE is required")
		os.Exit(2)
	}
	interval := 15 * time.Second
	if raw := strings.TrimSpace(os.Getenv("RECONCILE_INTERVAL")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed < time.Second || parsed > 10*time.Minute {
			slog.Error("invalid RECONCILE_INTERVAL")
			os.Exit(2)
		}
		interval = parsed
	}
	encoded, err := os.ReadFile(keyFile)
	if err != nil {
		slog.Error("read signing key", "error", err)
		os.Exit(2)
	}
	key, err := parsePrivateKey(strings.TrimSpace(string(encoded)))
	if err != nil {
		slog.Error("invalid signing key")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	var store *postgres.Store
	if passwordFile := os.Getenv("DATABASE_PASSWORD_FILE"); passwordFile != "" {
		store, err = postgres.OpenWithPasswordFile(ctx, os.Getenv("DATABASE_URL"), passwordFile)
	} else {
		store, err = postgres.Open(ctx, os.Getenv("DATABASE_URL"))
	}
	if err != nil {
		slog.Error("open database failed")
		os.Exit(1)
	}
	defer store.Close()
	reconcile := func() {
		gateways, err := store.GatewaysNeedingSnapshot(ctx)
		if err != nil {
			slog.Error("list gateways", "error", err)
			return
		}
		for _, gatewayID := range gateways {
			generation, err := store.NextGatewaySnapshotGeneration(ctx, gatewayID)
			if err == nil {
				err = store.EnqueueCurrentGatewaySnapshot(ctx, gatewayID, generation, key)
			}
			if err != nil {
				slog.Error("publish snapshot", "gateway_id", gatewayID, "error", err)
			} else {
				slog.Info("snapshot queued", "gateway_id", gatewayID, "generation", generation)
			}
		}
	}
	reconcile()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reconcile()
		}
	}
}

func parsePrivateKey(value string) (ed25519.PrivateKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) != ed25519.PrivateKeySize {
		return nil, errors.New("expected base64 Ed25519 private key")
	}
	return ed25519.PrivateKey(decoded), nil
}
