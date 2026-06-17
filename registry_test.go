package registry

import (
	"context"
	"errors"
	"testing"
)

// mockClient 是用于测试的 mock 实现
type mockClient struct {
	name        string
	healthErr   error
	closeErr    error
	closeCalled bool
}

func (m *mockClient) HealthCheck(ctx context.Context) error {
	return m.healthErr
}

func (m *mockClient) Close() error {
	m.closeCalled = true
	return m.closeErr
}

// mockProvider 是用于测试的 mock 实现
type mockProvider struct {
	name     string
	clients  map[string]*mockClient
	buildErr error
}

func (p *mockProvider) Name() string {
	return p.name
}

func (p *mockProvider) Build(instanceName string, cfg map[string]any) (Client, error) {
	if p.buildErr != nil {
		return nil, p.buildErr
	}
	if p.clients == nil {
		p.clients = make(map[string]*mockClient)
	}
	if c, ok := p.clients[instanceName]; ok {
		return c, nil
	}
	c := &mockClient{name: p.name + "/" + instanceName}
	p.clients[instanceName] = c
	return c, nil
}

func (p *mockProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (p *mockProvider) Close() error {
	return nil
}

func TestRegistry_Get_LazySingleton(t *testing.T) {
	Reset()

	provider := &mockProvider{name: "mock"}
	Register(provider)

	// 同一 instanceName 应返回同一实例
	inst1, err := Get("mock", "default")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	inst2, err := Get("mock", "default")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if inst1 != inst2 {
		t.Error("expected same instance for same name")
	}

	// 不同 instanceName 应返回不同实例
	inst3, err := Get("mock", "cache")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if inst1 == inst3 {
		t.Error("expected different instance for different name")
	}
}

func TestRegistry_Get_ProviderNotFound(t *testing.T) {
	Reset()

	_, err := Get("nonexistent", "default")
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
	if !errors.Is(err, ErrProviderNotFound) {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestRegistry_Register(t *testing.T) {
	Reset()

	provider := &mockProvider{name: "test"}
	Register(provider)

	client, err := Get("test", "default")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	mockClient, ok := client.(*mockClient)
	if !ok {
		t.Fatalf("expected *mockClient, got %T", client)
	}

	if mockClient.name != "test/default" {
		t.Errorf("expected name 'test/default', got %s", mockClient.name)
	}
}
