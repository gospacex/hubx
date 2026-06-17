package registry

import (
	"context"
	"testing"
)

func TestInitCore(t *testing.T) {
	Reset()

	provider := &mockProvider{name: "test"}
	Register(provider)

	err := InitCore(context.Background(), []string{"test:default"})
	if err != nil {
		t.Errorf("InitCore failed: %v", err)
	}

	// 再次获取应该用已初始化的实例
	client, err := Get("test", "default")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	mockClient, ok := client.(*mockClient)
	if !ok {
		t.Fatalf("expected *mockClient")
	}

	// 验证实例确实已创建
	if mockClient.name != "test/default" {
		t.Errorf("expected 'test/default', got %s", mockClient.name)
	}
}

func TestInitCore_InvalidFormat(t *testing.T) {
	Reset()

	err := InitCore(context.Background(), []string{"invalid-format"})
	if err == nil {
		t.Error("expected error for invalid format")
	}
}

func TestInitCore_ProviderNotFound(t *testing.T) {
	Reset()

	err := InitCore(context.Background(), []string{"nonexistent:default"})
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
}

func TestShutdown(t *testing.T) {
	Reset()

	provider := &mockProvider{name: "test"}
	Register(provider)

	// 初始化一个实例
	_, err := Get("test", "default")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// 注册关闭钩子
	hookCalled := false
	RegisterShutdownHook("test-hook", func(ctx context.Context) error {
		hookCalled = true
		return nil
	})

	err = Shutdown(context.Background())
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	if !hookCalled {
		t.Error("expected hook to be called")
	}

	// 关闭后再次获取应该报错
	_, err = Get("test", "default")
	if err == nil {
		t.Error("expected error after shutdown")
	}
}

func TestRegisterShutdownHook(t *testing.T) {
	Reset()

	calls := []string{}
	RegisterShutdownHook("hook1", func(ctx context.Context) error {
		calls = append(calls, "hook1")
		return nil
	})
	RegisterShutdownHook("hook2", func(ctx context.Context) error {
		calls = append(calls, "hook2")
		return nil
	})

	Shutdown(context.Background())

	// hook 应该按逆序执行
	if len(calls) != 2 {
		t.Errorf("expected 2 calls, got %d", len(calls))
	}
	if calls[0] != "hook2" || calls[1] != "hook1" {
		t.Errorf("expected [hook2, hook1], got %v", calls)
	}
}
