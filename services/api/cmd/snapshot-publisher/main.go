package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"personalvpn/services/api/internal/postgres"
)

func main() {
	getenv := os.Getenv
	databaseURL, gatewayID, generationRaw, keyFile := getenv("DATABASE_URL"), getenv("GATEWAY_ID"), getenv("SNAPSHOT_GENERATION"), getenv("SNAPSHOT_SIGNING_KEY_FILE")
	generation, err := strconv.ParseInt(strings.TrimSpace(generationRaw), 10, 64)
	if generationRaw != "" && err != nil || gatewayID == "" || keyFile == "" {
		slog.Error("invalid snapshot publisher configuration")
		os.Exit(2)
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
	store, err := postgres.Open(context.Background(), databaseURL)
	if err != nil {
		slog.Error("open database failed")
		os.Exit(1)
	}
	defer store.Close()
	if strings.TrimSpace(generationRaw) == "" {
		generation, err = store.NextGatewaySnapshotGeneration(context.Background(), gatewayID)
		if err != nil {
			slog.Error("get next snapshot generation", "error", err)
			os.Exit(1)
		}
	}
	if err := store.EnqueueCurrentGatewaySnapshot(context.Background(), gatewayID, generation, key); err != nil {
		slog.Error("publish gateway snapshot", "error", err)
		os.Exit(1)
	}
	slog.Info("gateway snapshot published", "gateway_id", gatewayID, "generation", generation)
}

func parsePrivateKey(value string) (ed25519.PrivateKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) != ed25519.PrivateKeySize {
		return nil, errors.New("expected base64 Ed25519 private key")
	}
	return ed25519.PrivateKey(decoded), nil
}
