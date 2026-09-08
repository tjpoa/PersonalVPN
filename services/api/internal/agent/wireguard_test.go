package agent

import (
	"context"
	"errors"
	"testing"
)

type fakeRunner struct {
	name string
	args [][]string
	err  error
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) error {
	f.name = name
	f.args = append(f.args, append([]string(nil), args...))
	return f.err
}

func TestWireGuardBackendBuildsFixedArguments(t *testing.T) {
	runner := &fakeRunner{}
	backend := WireGuardBackend{InterfaceName: "wg0", Runner: runner}
	operation := Operation{Kind: OperationAdd, Peer: Peer{PublicKey: agentKey(1), IPv4: "10.70.0.2/32", IPv6: "fd70::2/128"}}
	if err := backend.Apply(context.Background(), []Operation{operation}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	want := []string{"set", "wg0", "peer", agentKey(1), "allowed-ips", "10.70.0.2/32,fd70::2/128"}
	if runner.name != "wg" || len(runner.args) != 1 || len(runner.args[0]) != len(want) {
		t.Fatalf("runner call = %q %+v", runner.name, runner.args)
	}
	for index := range want {
		if runner.args[0][index] != want[index] {
			t.Errorf("argument %d = %q, want %q", index, runner.args[0][index], want[index])
		}
	}
}

func TestWireGuardBackendRemoveCannotMutateAddresses(t *testing.T) {
	runner := &fakeRunner{}
	backend := WireGuardBackend{InterfaceName: "wg0", Runner: runner}
	err := backend.Apply(context.Background(), []Operation{{Kind: OperationRemove, Peer: Peer{PublicKey: agentKey(1), IPv4: "10.70.0.2/32"}}})
	if !errors.Is(err, ErrInvalidOperation) {
		t.Fatalf("Apply() error = %v, want ErrInvalidOperation", err)
	}
	if len(runner.args) != 0 {
		t.Fatal("invalid remove operation reached runner")
	}
}

func TestWireGuardBackendRejectsShellLikeInterfaceName(t *testing.T) {
	runner := &fakeRunner{}
	backend := WireGuardBackend{InterfaceName: "wg0;rm", Runner: runner}
	err := backend.Apply(context.Background(), []Operation{{Kind: OperationAdd, Peer: Peer{PublicKey: agentKey(1), IPv4: "10.70.0.2/32", IPv6: "fd70::2/128"}}})
	if !errors.Is(err, ErrInvalidOperation) {
		t.Fatalf("Apply() error = %v, want ErrInvalidOperation", err)
	}
}

func TestWireGuardBackendStopsAfterRunnerFailure(t *testing.T) {
	runner := &fakeRunner{err: errors.New("simulated failure")}
	backend := WireGuardBackend{InterfaceName: "wg0", Runner: runner}
	operations := []Operation{
		{Kind: OperationAdd, Peer: Peer{PublicKey: agentKey(1), IPv4: "10.70.0.2/32", IPv6: "fd70::2/128"}},
		{Kind: OperationAdd, Peer: Peer{PublicKey: agentKey(2), IPv4: "10.70.0.3/32", IPv6: "fd70::3/128"}},
	}
	if err := backend.Apply(context.Background(), operations); err == nil {
		t.Fatal("Apply() accepted runner failure")
	}
	if len(runner.args) != 1 {
		t.Fatalf("runner received %d operations after failure, want 1", len(runner.args))
	}
}
