package agent

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"time"
)

const maxPeersPerSnapshot = 100000

var (
	ErrInvalidSnapshot  = errors.New("invalid gateway snapshot")
	ErrInvalidSignature = errors.New("invalid gateway snapshot signature")
)

type Peer struct {
	PublicKey string `json:"publicKey"`
	IPv4      string `json:"ipv4"`
	IPv6      string `json:"ipv6"`
}

type Snapshot struct {
	GatewayID  string    `json:"gatewayId"`
	Generation int64     `json:"generation"`
	ExpiresAt  time.Time `json:"expiresAt"`
	Peers      []Peer    `json:"peers"`
	Signature  []byte    `json:"signature,omitempty"`
}

type OperationKind string

const (
	OperationAdd    OperationKind = "add"
	OperationUpdate OperationKind = "update"
	OperationRemove OperationKind = "remove"
)

type Operation struct {
	Kind OperationKind `json:"kind"`
	Peer Peer          `json:"peer"`
}

func (s Snapshot) Validate(now time.Time, gatewayID string) error {
	if s.GatewayID == "" || s.GatewayID != gatewayID || s.Generation <= 0 || !s.ExpiresAt.After(now) {
		return ErrInvalidSnapshot
	}
	if len(s.Peers) > maxPeersPerSnapshot {
		return ErrInvalidSnapshot
	}
	seenKeys := make(map[string]struct{}, len(s.Peers))
	seenIPv4 := make(map[string]struct{}, len(s.Peers))
	seenIPv6 := make(map[string]struct{}, len(s.Peers))
	for _, peer := range s.Peers {
		if !validPublicKey(peer.PublicKey) || !validHostPrefix(peer.IPv4, 4) || !validHostPrefix(peer.IPv6, 6) {
			return ErrInvalidSnapshot
		}
		if _, exists := seenKeys[peer.PublicKey]; exists {
			return ErrInvalidSnapshot
		}
		if _, exists := seenIPv4[peer.IPv4]; exists {
			return ErrInvalidSnapshot
		}
		if _, exists := seenIPv6[peer.IPv6]; exists {
			return ErrInvalidSnapshot
		}
		seenKeys[peer.PublicKey] = struct{}{}
		seenIPv4[peer.IPv4] = struct{}{}
		seenIPv6[peer.IPv6] = struct{}{}
	}
	return nil
}

func (s Snapshot) Verify(now time.Time, gatewayID string, publicKey ed25519.PublicKey) error {
	if err := s.Validate(now, gatewayID); err != nil {
		return err
	}
	if len(publicKey) != ed25519.PublicKeySize || len(s.Signature) != ed25519.SignatureSize {
		return ErrInvalidSignature
	}
	content, err := s.signingBytes()
	if err != nil || !ed25519.Verify(publicKey, content, s.Signature) {
		return ErrInvalidSignature
	}
	return nil
}

func Sign(snapshot Snapshot, privateKey ed25519.PrivateKey) (Snapshot, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return Snapshot{}, ErrInvalidSignature
	}
	snapshot.Signature = nil
	content, err := snapshot.signingBytes()
	if err != nil {
		return Snapshot{}, fmt.Errorf("encode snapshot: %w", err)
	}
	snapshot.Signature = ed25519.Sign(privateKey, content)
	return snapshot, nil
}

func Diff(desired Snapshot, observed map[string]Peer) ([]Operation, error) {
	// Expiry is enforced by Verify before Diff; this pure function validates shape only.
	if err := desired.Validate(time.Time{}, desired.GatewayID); err != nil {
		return nil, err
	}
	desiredByKey := make(map[string]Peer, len(desired.Peers))
	for _, peer := range desired.Peers {
		desiredByKey[peer.PublicKey] = peer
	}
	operations := make([]Operation, 0)
	for key, peer := range desiredByKey {
		current, exists := observed[key]
		if !exists {
			operations = append(operations, Operation{Kind: OperationAdd, Peer: peer})
		} else if current.IPv4 != peer.IPv4 || current.IPv6 != peer.IPv6 {
			operations = append(operations, Operation{Kind: OperationUpdate, Peer: peer})
		}
	}
	for key, peer := range observed {
		if _, exists := desiredByKey[key]; !exists {
			operations = append(operations, Operation{Kind: OperationRemove, Peer: Peer{PublicKey: peer.PublicKey}})
		}
	}
	sort.Slice(operations, func(i, j int) bool {
		if operations[i].Peer.PublicKey == operations[j].Peer.PublicKey {
			return operations[i].Kind < operations[j].Kind
		}
		return operations[i].Peer.PublicKey < operations[j].Peer.PublicKey
	})
	return operations, nil
}

func (s Snapshot) signingBytes() ([]byte, error) {
	unsigned := s
	unsigned.Signature = nil
	return json.Marshal(unsigned)
}

func validPublicKey(value string) bool {
	decoded, err := base64.StdEncoding.DecodeString(value)
	return err == nil && len(decoded) == 32 && base64.StdEncoding.EncodeToString(decoded) == value
}

func validHostPrefix(value string, family int) bool {
	prefix, err := netip.ParsePrefix(value)
	if err != nil || prefix.Bits() != prefix.Addr().BitLen() {
		return false
	}
	return (family == 4 && prefix.Addr().BitLen() == 32) || (family == 6 && prefix.Addr().BitLen() == 128)
}
