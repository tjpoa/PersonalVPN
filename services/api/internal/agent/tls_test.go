package agent

import (
	"errors"
	"testing"
)

func TestTLSConfigRequiresAllFiles(t *testing.T) {
	if _, err := ClientTLSConfig(TLSFiles{}); !errors.Is(err, ErrInvalidTLSConfiguration) {
		t.Fatalf("ClientTLSConfig() error = %v, want ErrInvalidTLSConfiguration", err)
	}
	if _, err := ServerTLSConfig(TLSFiles{}); !errors.Is(err, ErrInvalidTLSConfiguration) {
		t.Fatalf("ServerTLSConfig() error = %v, want ErrInvalidTLSConfiguration", err)
	}
}
