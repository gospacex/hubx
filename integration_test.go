package registry

import (
	"context"
	"sync"
	"testing"
)

// stubConfigLoader is a ConfigLoader implementation for tests. It returns a
// canned config map for any (provider, instance) pair without reading from
// disk. Using a stub here avoids the import cycle that would arise if
// integration_test.go imported configx/hubx (which itself imports hubx).
type stubConfigLoader struct {
	mu        sync.Mutex
	cfg       map[string]any
	watchers  []func(provider, instance string)
	closed    bool
	loadErr   error
	loadCalls int
}

func (s *stubConfigLoader) Load(provider, instance string) (map[string]any, error) {
	s.mu.Lock()
	s.loadCalls++
	s.mu.Unlock()
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	if s.cfg == nil {
		return map[string]any{"config": map[string]any{}}, nil
	}
	return s.cfg, nil
}

func (s *stubConfigLoader) Watch(cb func(provider, instance string)) {
	if cb == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.watchers = append(s.watchers, cb)
}

func (s *stubConfigLoader) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func TestRegistry_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	Reset()
	t.Cleanup(func() { Shutdown(context.Background()) })

	loader := &stubConfigLoader{
		cfg: map[string]any{"config": map[string]any{"enabled": true}},
	}
	SetConfigLoader(loader)

	provider := &mockProvider{name: "mock"}
	Register(provider)

	// Get instance — singleton.
	client1, err := Get("mock", "default")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	client2, err := Get("mock", "default")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if client1 != client2 {
		t.Error("expected singleton")
	}

	// The stub loader must have been called once per Get (the second Get is
	// a cache hit and does not re-invoke the loader).
	if loader.loadCalls < 1 {
		t.Errorf("expected at least 1 Load call, got %d", loader.loadCalls)
	}

	// Health check across the registry.
	results := HealthCheckAllInstances(context.Background())
	if len(results) != 1 {
		t.Errorf("expected 1 health result, got %d", len(results))
	}

	// Loader.Close() should be invoked when SetConfigLoader is replaced.
	if err := loader.Close(); err != nil {
		t.Errorf("loader close: %v", err)
	}
}

func TestInitCore_Integration(t *testing.T) {
	Reset()
	t.Cleanup(func() { Shutdown(context.Background()) })

	provider := &mockProvider{name: "test"}
	Register(provider)

	// InitCore initializes the listed core instances.
	err := InitCore(context.Background(), []string{"test:default"})
	if err != nil {
		t.Errorf("InitCore failed: %v", err)
	}

	client, err := Get("test", "default")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	mockClient, ok := client.(*mockClient)
	if !ok {
		t.Fatalf("expected *mockClient")
	}

	if mockClient.name != "test/default" {
		t.Errorf("expected 'test/default', got %s", mockClient.name)
	}
}

func TestShutdown_Integration(t *testing.T) {
	Reset()
	t.Cleanup(func() { Shutdown(context.Background()) })

	provider := &mockProvider{name: "test"}
	Register(provider)

	// Build the instance up front.
	_, err := Get("test", "default")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Register a shutdown hook and verify it fires.
	hookCalled := false
	RegisterShutdownHook("test-hook", func(ctx context.Context) error {
		hookCalled = true
		return nil
	})

	if err := Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
	if !hookCalled {
		t.Error("expected hook to be called")
	}

	// After Shutdown, Get must return an error.
	if _, err := Get("test", "default"); err == nil {
		t.Error("expected error after shutdown")
	}
}

func TestConfigLoaderReplacement_ClosesPrevious(t *testing.T) {
	Reset()
	t.Cleanup(func() { Shutdown(context.Background()) })

	old := &stubConfigLoader{}
	SetConfigLoader(old)
	// Replace; the old loader should be closed.
	newer := &stubConfigLoader{}
	SetConfigLoader(newer)
	if !old.closed {
		t.Error("expected previous loader to be closed after replacement")
	}
}
