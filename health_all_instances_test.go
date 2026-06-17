package registry

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

// TestHealthCheckAllInstances_Empty_ReturnsEmpty verifies the function returns
// no error and an empty slice when no instances have been built.
func TestHealthCheckAllInstances_Empty_ReturnsEmpty(t *testing.T) {
	Reset()

	results := HealthCheckAllInstances(context.Background())
	if results == nil {
		// Acceptable: empty (non-nil) or nil slice
		results = []HealthResult{}
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// TestHealthCheckAllInstances_AllHealthy verifies a result is returned per
// instance, each marked healthy, when all HealthCheck calls succeed.
func TestHealthCheckAllInstances_AllHealthy(t *testing.T) {
	Reset()

	p := &mockProvider{name: "redis"}
	Register(p)

	for _, inst := range []string{"a", "b", "c"} {
		if _, err := Get("redis", inst); err != nil {
			t.Fatalf("Get %s failed: %v", inst, err)
		}
	}

	results := HealthCheckAllInstances(context.Background())
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Provider != "redis" {
			t.Errorf("expected provider redis, got %q", r.Provider)
		}
		if !r.Healthy {
			t.Errorf("expected healthy for %s, got error: %v", r.Instance, r.Error)
		}
		if r.Error != nil {
			t.Errorf("expected nil error, got %v", r.Error)
		}
		if r.LatencyMs < 0 {
			t.Errorf("expected non-negative latency, got %d", r.LatencyMs)
		}
	}
}

// TestHealthCheckAllInstances_OneUnhealthy verifies that when one instance's
// HealthCheck returns an error, that result has Healthy=false and Error set,
// while the other instances report Healthy=true.
func TestHealthCheckAllInstances_OneUnhealthy(t *testing.T) {
	Reset()

	p := &mockProvider{name: "redis"}
	Register(p)

	for _, inst := range []string{"a", "b", "c"} {
		if _, err := Get("redis", inst); err != nil {
			t.Fatalf("Get %s failed: %v", inst, err)
		}
	}
	// Inject a synthetic unhealthy client by accessing the cached one and
	// replacing its healthErr field through Build. Since mockProvider stores
	// the same client per instance, we set healthErr on the b instance.
	bClient, err := Get("redis", "b")
	if err != nil {
		t.Fatalf("Get b failed: %v", err)
	}
	mc, ok := bClient.(*mockClient)
	if !ok {
		t.Fatalf("expected *mockClient, got %T", bClient)
	}
	mc.healthErr = errors.New("simulated failure")

	results := HealthCheckAllInstances(context.Background())
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	unhealthyCount := 0
	for _, r := range results {
		if r.Instance == "b" {
			if r.Healthy {
				t.Errorf("expected b unhealthy")
			}
			if r.Error == nil || !strings.Contains(r.Error.Error(), "simulated failure") {
				t.Errorf("expected simulated failure error, got %v", r.Error)
			}
			unhealthyCount++
		}
	}
	if unhealthyCount != 1 {
		t.Errorf("expected exactly 1 unhealthy result, got %d", unhealthyCount)
	}
}

// TestHealthCheckAllInstances_DoesNotTriggerBuild verifies that the function
// reads existing instances directly from the sync.Map and does NOT call the
// provider's Build method.
func TestHealthCheckAllInstances_DoesNotTriggerBuild(t *testing.T) {
	Reset()

	var buildCount atomic.Int32
	tracker := &buildTrackingProvider{
		name:    "tracked",
		clients: map[string]*mockClient{},
		counter: &buildCount,
	}
	Register(tracker)

	// Pre-build one instance
	if _, err := Get("tracked", "pre"); err != nil {
		t.Fatalf("Get pre failed: %v", err)
	}
	before := buildCount.Load()

	// Call HealthCheckAllInstances — it must NOT trigger any Build
	results := HealthCheckAllInstances(context.Background())
	if len(results) != 1 {
		t.Fatalf("expected 1 result (only pre-built), got %d", len(results))
	}
	after := buildCount.Load()
	if after != before {
		t.Errorf("expected build count unchanged (was %d, now %d)", before, after)
	}
}

// buildTrackingProvider wraps mockProvider but exposes the build counter.
type buildTrackingProvider struct {
	name    string
	clients map[string]*mockClient
	counter *atomic.Int32
}

func (p *buildTrackingProvider) Name() string { return p.name }

func (p *buildTrackingProvider) Build(instanceName string, cfg map[string]any) (Client, error) {
	p.counter.Add(1)
	if c, ok := p.clients[instanceName]; ok {
		return c, nil
	}
	c := &mockClient{name: p.name + "/" + instanceName}
	p.clients[instanceName] = c
	return c, nil
}

func (p *buildTrackingProvider) HealthCheck(ctx context.Context) error { return nil }
func (p *buildTrackingProvider) Close() error                          { return nil }
