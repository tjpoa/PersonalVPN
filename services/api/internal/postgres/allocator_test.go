package postgres

import (
	"errors"
	"net/netip"
	"testing"

	"personalvpn/services/api/internal/control"
)

func TestFirstAvailableAddress(t *testing.T) {
	used := map[netip.Addr]struct{}{
		netip.MustParseAddr("10.70.0.1"): {},
		netip.MustParseAddr("10.70.0.2"): {},
	}
	address, err := firstAvailableAddress("10.70.0.0/29", used)
	if err != nil {
		t.Fatalf("firstAvailableAddress() error = %v", err)
	}
	if got, want := address.String(), "10.70.0.3"; got != want {
		t.Fatalf("firstAvailableAddress() = %s, want %s", got, want)
	}
}

func TestFirstAvailableIPv6Address(t *testing.T) {
	address, err := firstAvailableAddress("fd70::/64", nil)
	if err != nil {
		t.Fatalf("firstAvailableAddress() error = %v", err)
	}
	if got, want := address.String(), "fd70::2"; got != want {
		t.Fatalf("firstAvailableAddress() = %s, want %s", got, want)
	}
}

func TestFirstAvailableAddressExhausted(t *testing.T) {
	used := map[netip.Addr]struct{}{netip.MustParseAddr("192.0.2.1"): {}}
	_, err := firstAvailableAddress("192.0.2.0/31", used)
	if !errors.Is(err, control.ErrCapacityExhausted) {
		t.Fatalf("firstAvailableAddress() error = %v, want ErrCapacityExhausted", err)
	}
}
