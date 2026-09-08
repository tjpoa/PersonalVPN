package agent

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

var ErrInvalidOperation = errors.New("invalid gateway operation")

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type ProcessRunner struct{}

func (ProcessRunner) Run(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

type WireGuardBackend struct {
	InterfaceName string
	Runner        CommandRunner
}

func (b WireGuardBackend) Apply(ctx context.Context, operations []Operation) error {
	if b.InterfaceName == "" || b.Runner == nil || len(operations) > maxPeersPerSnapshot {
		return ErrInvalidOperation
	}
	for _, operation := range operations {
		args, err := operationArguments(b.InterfaceName, operation)
		if err != nil {
			return err
		}
		if err := b.Runner.Run(ctx, "wg", args...); err != nil {
			return fmt.Errorf("apply WireGuard operation %s: %w", operation.Kind, err)
		}
	}
	return nil
}

func operationArguments(interfaceName string, operation Operation) ([]string, error) {
	if interfaceName == "" || interfaceName != sanitizeInterfaceName(interfaceName) {
		return nil, ErrInvalidOperation
	}
	if !validPublicKey(operation.Peer.PublicKey) {
		return nil, ErrInvalidOperation
	}
	switch operation.Kind {
	case OperationRemove:
		if operation.Peer.IPv4 != "" || operation.Peer.IPv6 != "" {
			return nil, ErrInvalidOperation
		}
		return []string{"set", interfaceName, "peer", operation.Peer.PublicKey, "remove"}, nil
	case OperationAdd, OperationUpdate:
		if !validHostPrefix(operation.Peer.IPv4, 4) || !validHostPrefix(operation.Peer.IPv6, 6) {
			return nil, ErrInvalidOperation
		}
		return []string{
			"set", interfaceName, "peer", operation.Peer.PublicKey,
			"allowed-ips", operation.Peer.IPv4 + "," + operation.Peer.IPv6,
		}, nil
	default:
		return nil, ErrInvalidOperation
	}
}

func sanitizeInterfaceName(value string) string {
	for _, character := range value {
		if !(character >= 'a' && character <= 'z') &&
			!(character >= 'A' && character <= 'Z') &&
			!(character >= '0' && character <= '9') && character != '_' && character != '-' {
			return ""
		}
	}
	return value
}
