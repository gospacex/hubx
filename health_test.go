package registry

import (
	"context"
	"testing"
)

func TestRegistryHealthChecker_Check(t *testing.T) {
	Reset()

	provider := &mockProvider{name: "test"}
	Register(provider)

	checker := &RegistryHealthChecker{}
	results := checker.Check(context.Background())

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if results[0].Provider != "test" {
		t.Errorf("expected provider 'test', got %s", results[0].Provider)
	}
}

func TestInstanceHealthChecker_Check(t *testing.T) {
	Reset()

	provider := &mockProvider{name: "test"}
	Register(provider)

	checker := &InstanceHealthChecker{
		providerName: "test",
		instanceName: "default",
	}

	result := checker.Check(context.Background())

	if !result.Healthy {
		t.Errorf("expected healthy, got error: %v", result.Error)
	}

	if result.Instance != "default" {
		t.Errorf("expected instance 'default', got %s", result.Instance)
	}
}

func TestInstanceHealthChecker_Check_NotFound(t *testing.T) {
	Reset()

	checker := &InstanceHealthChecker{
		providerName: "nonexistent",
		instanceName: "default",
	}

	result := checker.Check(context.Background())

	if result.Healthy {
		t.Error("expected unhealthy for nonexistent provider")
	}
}

func TestHealthCheckAllInstances(t *testing.T) {
	Reset()

	provider1 := &mockProvider{name: "test1"}
	provider2 := &mockProvider{name: "test2"}
	Register(provider1)
	Register(provider2)

	// Build both instances so they land in the instances sync.Map.
	if _, err := Get("test1", "default"); err != nil {
		t.Fatalf("Get test1: %v", err)
	}
	if _, err := Get("test2", "default"); err != nil {
		t.Fatalf("Get test2: %v", err)
	}

	results := HealthCheckAllInstances(context.Background())

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestHealthCheckAllInstances_Empty(t *testing.T) {
	Reset()
	results := HealthCheckAllInstances(context.Background())
	if results == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results when no instances built, got %d", len(results))
	}
}

func TestHealthResult_Latency(t *testing.T) {
	Reset()

	provider := &mockProvider{name: "slow"}
	Register(provider)

	checker := &InstanceHealthChecker{
		providerName: "slow",
		instanceName: "default",
	}

	// mockClient.HealthCheck 已经很快，这里只验证 latency 字段有值
	result := checker.Check(context.Background())

	if result.LatencyMs < 0 {
		t.Errorf("expected non-negative latency, got %d", result.LatencyMs)
	}
}
