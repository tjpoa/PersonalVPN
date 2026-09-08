package agent

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func agentKey(seed byte) string {
	value := make([]byte, 32)
	for index := range value {
		value[index] = seed
	}
	return base64.StdEncoding.EncodeToString(value)
}

func validSnapshot() Snapshot {
	return Snapshot{
		GatewayID: "gateway-1", Generation: 1, ExpiresAt: time.Date(2026, 9, 2, 13, 0, 0, 0, time.UTC),
		Peers: []Peer{{PublicKey: agentKey(1), IPv4: "10.70.0.2/32", IPv6: "fd70::2/128"}},
	}
}

func TestSnapshotSignatureAndTamperDetection(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	snapshot, err := Sign(validSnapshot(), privateKey)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	if err := snapshot.Verify(now, "gateway-1", publicKey); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	snapshot.Peers[0].IPv4 = "10.70.0.9/32"
	if !errors.Is(snapshot.Verify(now, "gateway-1", publicKey), ErrInvalidSignature) {
		t.Fatal("tampered snapshot was accepted")
	}
}

func TestSnapshotRejectsDuplicatesAndExpiry(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	snapshot := validSnapshot()
	snapshot.Peers = append(snapshot.Peers, Peer{PublicKey: agentKey(1), IPv4: "10.70.0.3/32", IPv6: "fd70::3/128"})
	if !errors.Is(snapshot.Validate(now, "gateway-1"), ErrInvalidSnapshot) {
		t.Fatal("duplicate key accepted")
	}
	snapshot = validSnapshot()
	snapshot.ExpiresAt = now
	if !errors.Is(snapshot.Validate(now, "gateway-1"), ErrInvalidSnapshot) {
		t.Fatal("expired snapshot accepted")
	}
}

func TestDiffIsDeterministicAndTyped(t *testing.T) {
	desired := validSnapshot()
	desired.Peers = append(desired.Peers, Peer{PublicKey: agentKey(2), IPv4: "10.70.0.3/32", IPv6: "fd70::3/128"})
	observed := map[string]Peer{agentKey(1): {PublicKey: agentKey(1), IPv4: "10.70.0.9/32", IPv6: "fd70::9/128"}, agentKey(3): {PublicKey: agentKey(3), IPv4: "10.70.0.4/32", IPv6: "fd70::4/128"}}
	operations, err := Diff(desired, observed)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if len(operations) != 3 || operations[0].Kind != OperationUpdate || operations[1].Kind != OperationAdd || operations[2].Kind != OperationRemove {
		t.Fatalf("Diff() = %+v", operations)
	}
	if operations[2].Peer.IPv4 != "" || operations[2].Peer.IPv6 != "" {
		t.Fatal("remove operation contains address mutation")
	}
}
