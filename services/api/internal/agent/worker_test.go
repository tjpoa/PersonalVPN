package agent

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

type fakeSource struct {
	delivery Delivery
	claimed  bool
	acked    bool
	retried  bool
}

func (s *fakeSource) Claim(context.Context) (Delivery, error) {
	if s.claimed {
		return Delivery{}, ErrNoDelivery
	}
	s.claimed = true
	return s.delivery, nil
}
func (s *fakeSource) Ack(context.Context, string, int64) error { s.acked = true; return nil }
func (s *fakeSource) Retry(context.Context, string, time.Time, string) error {
	s.retried = true
	return nil
}

type fakeApplier struct {
	operations []Operation
	err        error
}

func workerPublicKey(seed byte) string {
	key := make([]byte, ed25519.PublicKeySize)
	for i := range key {
		key[i] = seed
	}
	return base64.StdEncoding.EncodeToString(key)
}

func (a *fakeApplier) Apply(_ context.Context, operations []Operation) error {
	a.operations = operations
	return a.err
}

func TestWorkerAppliesAndAcknowledgesVerifiedSnapshot(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	now := time.Unix(1_700_000_000, 0).UTC()
	snapshot := Snapshot{GatewayID: "gw-1", Generation: 1, ExpiresAt: now.Add(time.Hour), Peers: []Peer{{PublicKey: workerPublicKey(7), IPv4: "10.0.0.2/32", IPv6: "fd00::2/128"}}}
	snapshot, err := Sign(snapshot, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := snapshot.signingBytes()
	source := &fakeSource{delivery: Delivery{ID: "d1", Generation: 1, Payload: payload, Signature: snapshot.Signature}}
	applier := &fakeApplier{}
	worker := Worker{Source: source, Applier: applier, GatewayID: "gw-1", VerifyKey: publicKey, Now: func() time.Time { return now }}
	if err := worker.ApplyOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !source.acked || source.retried || len(applier.operations) != 1 || worker.Current.Generation != 1 {
		t.Fatalf("unexpected worker state: %+v %+v", source, worker.Current)
	}
}

func TestWorkerRetriesInvalidSnapshotAndApplyFailure(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	now := time.Unix(1_700_000_000, 0).UTC()
	snapshot, _ := Sign(Snapshot{GatewayID: "gw-1", Generation: 1, ExpiresAt: now.Add(time.Hour)}, privateKey)
	payload, _ := snapshot.signingBytes()
	source := &fakeSource{delivery: Delivery{ID: "d1", Generation: 1, Payload: payload, Signature: snapshot.Signature}}
	applier := &fakeApplier{err: errors.New("wg unavailable")}
	worker := Worker{Source: source, Applier: applier, GatewayID: "gw-1", VerifyKey: publicKey, Now: func() time.Time { return now }}
	if err := worker.ApplyOnce(context.Background()); err == nil || !source.retried || source.acked {
		t.Fatalf("expected retry, source=%+v err=%v", source, err)
	}
}

func TestWorkerRunStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	source := &fakeSource{}
	worker := Worker{Source: source, Applier: &fakeApplier{}, GatewayID: "gw-1", VerifyKey: make(ed25519.PublicKey, ed25519.PublicKeySize)}
	if err := worker.Run(ctx, time.Millisecond); err == nil {
		t.Fatal("Run() error = nil, want context cancellation")
	}
}
