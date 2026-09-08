package agent

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrNoDelivery = errors.New("no snapshot delivery available")

type Delivery struct {
	ID         string `json:"id"`
	Generation int64  `json:"generation"`
	Payload    []byte `json:"payload"`
	Signature  []byte `json:"signature"`
}

type DeliverySource interface {
	Claim(ctx context.Context) (Delivery, error)
	Ack(ctx context.Context, deliveryID string, generation int64) error
	Retry(ctx context.Context, deliveryID string, retryAt time.Time, reason string) error
}

type SnapshotApplier interface {
	Apply(ctx context.Context, operations []Operation) error
}

type Worker struct {
	Source       DeliverySource
	Applier      SnapshotApplier
	GatewayID    string
	VerifyKey    ed25519.PublicKey
	Now          func() time.Time
	Current      Snapshot
	CurrentPeers map[string]Peer
}

// Run continuously drains deliveries until the context is cancelled. Errors
// are retried with bounded backoff; delivery-level retry state remains owned by
// the control plane outbox.
func (w *Worker) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return ErrInvalidOperation
	}
	backoff := interval
	for {
		err := w.ApplyOnce(ctx)
		if errors.Is(err, ErrNoDelivery) {
			backoff = interval
		} else if err != nil {
			if backoff < 30*time.Second {
				backoff *= 2
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
			}
		} else {
			backoff = interval
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// ApplyOnce processes at most one delivery. A delivery is acknowledged only
// after validation and all WireGuard operations succeed.
func (w *Worker) ApplyOnce(ctx context.Context) error {
	if w.Source == nil || w.Applier == nil || w.GatewayID == "" || len(w.VerifyKey) != ed25519.PublicKeySize {
		return ErrInvalidOperation
	}
	delivery, err := w.Source.Claim(ctx)
	if err != nil {
		return err
	}
	now := time.Now
	if w.Now != nil {
		now = w.Now
	}
	var snapshot Snapshot
	if err := json.Unmarshal(delivery.Payload, &snapshot); err != nil {
		return w.retry(ctx, delivery, fmt.Errorf("decode snapshot: %w", err))
	}
	snapshot.Signature = delivery.Signature
	if snapshot.Generation != delivery.Generation {
		return w.retry(ctx, delivery, ErrInvalidSnapshot)
	}
	if err := snapshot.Verify(now().UTC(), w.GatewayID, w.VerifyKey); err != nil {
		return w.retry(ctx, delivery, err)
	}
	if snapshot.Generation <= w.Current.Generation {
		return w.Source.Ack(ctx, delivery.ID, delivery.Generation)
	}
	operations, err := Diff(snapshot, w.CurrentPeers)
	if err != nil {
		return w.retry(ctx, delivery, err)
	}
	if err := w.Applier.Apply(ctx, operations); err != nil {
		return w.retry(ctx, delivery, fmt.Errorf("apply snapshot: %w", err))
	}
	if err := w.Source.Ack(ctx, delivery.ID, delivery.Generation); err != nil {
		return err
	}
	w.Current = snapshot
	w.CurrentPeers = make(map[string]Peer, len(snapshot.Peers))
	for _, peer := range snapshot.Peers {
		w.CurrentPeers[peer.PublicKey] = peer
	}
	return nil
}

func (w *Worker) retry(ctx context.Context, delivery Delivery, cause error) error {
	if err := w.Source.Retry(ctx, delivery.ID, time.Now().UTC().Add(time.Minute), cause.Error()); err != nil {
		return err
	}
	return cause
}
