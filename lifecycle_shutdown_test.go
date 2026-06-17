package registry

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestShutdown_AllCloseSucceed_ReturnsNil verifies Shutdown returns nil when
// every hook and every provider.Close returns nil.
func TestShutdown_AllCloseSucceed_ReturnsNil(t *testing.T) {
	Reset()

	RegisterShutdownHook("ok-hook-1", func(ctx context.Context) error { return nil })
	RegisterShutdownHook("ok-hook-2", func(ctx context.Context) error { return nil })
	Register(&mockProvider{name: "ok-provider"})

	if err := Shutdown(context.Background()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// TestShutdown_SingleCloseFails_WrapsWithShutdownPrefix verifies that when
// exactly one close operation fails, Shutdown returns a wrapped error tagged
// with a `shutdown:` prefix and the caller can use errors.Is against
// ErrShutdownFailed.
func TestShutdown_SingleCloseFails_WrapsWithShutdownPrefix(t *testing.T) {
	Reset()

	wantErr := errors.New("boom-hook")
	RegisterShutdownHook("failing-hook", func(ctx context.Context) error { return wantErr })

	err := Shutdown(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrShutdownFailed) {
		t.Errorf("expected errors.Is(err, ErrShutdownFailed), got %v", err)
	}
	if !strings.Contains(err.Error(), "shutdown:") {
		t.Errorf("expected error to contain 'shutdown:' prefix, got %q", err.Error())
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected errors.Is(err, wantErr), got %v", err)
	}
}

// TestShutdown_MultipleCloseFail_ErrorsJoinPreservesAll verifies that when
// multiple close operations fail, every underlying error is reachable via
// errors.Is.
func TestShutdown_MultipleCloseFail_ErrorsJoinPreservesAll(t *testing.T) {
	Reset()

	errA := errors.New("err-A")
	errB := errors.New("err-B")
	errC := errors.New("err-C")

	RegisterShutdownHook("hook-a", func(ctx context.Context) error { return errA })
	RegisterShutdownHook("hook-b", func(ctx context.Context) error { return errB })
	RegisterShutdownHook("hook-c", func(ctx context.Context) error { return errC })

	err := Shutdown(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	for _, want := range []error{errA, errB, errC} {
		if !errors.Is(err, want) {
			t.Errorf("expected errors.Is(err, %v) == true, got %v", want, err)
		}
	}
}

// TestShutdown_SecondCall_ReturnsErrRegistryClosed verifies that the second
// call to Shutdown is idempotent and returns ErrRegistryClosed.
func TestShutdown_SecondCall_ReturnsErrRegistryClosed(t *testing.T) {
	Reset()

	if err := Shutdown(context.Background()); err != nil {
		t.Fatalf("first shutdown: %v", err)
	}
	err := Shutdown(context.Background())
	if !errors.Is(err, ErrRegistryClosed) {
		t.Errorf("expected ErrRegistryClosed, got %v", err)
	}
}
