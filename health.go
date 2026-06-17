package registry

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// HealthResult 健康检查结果
type HealthResult struct {
	Provider  string
	Instance  string
	Healthy   bool
	LatencyMs int64
	Error     error
}

// HealthChecker 健康检查接口
type HealthChecker interface {
	Check(ctx context.Context) HealthResult
}

// RegistryHealthChecker Registry 级别的健康检查
type RegistryHealthChecker struct{}

// Check 检查所有已初始化的实例
func (r *RegistryHealthChecker) Check(ctx context.Context) []HealthResult {
	var results []HealthResult

	// 检查所有 provider 的健康状态
	for name, provider := range Registry().providers {
		result := HealthResult{
			Provider: name,
			Healthy:  true,
		}

		start := time.Now()
		err := provider.HealthCheck(ctx)
		result.LatencyMs = time.Since(start).Milliseconds()

		if err != nil {
			result.Healthy = false
			result.Error = err
		}

		results = append(results, result)
	}

	return results
}

// InstanceHealthChecker 单个实例的健康检查
type InstanceHealthChecker struct {
	providerName string
	instanceName string
}

func (c *InstanceHealthChecker) Check(ctx context.Context) HealthResult {
	result := HealthResult{
		Provider: c.providerName,
		Instance: c.instanceName,
		Healthy:  true,
	}

	client, err := Get(c.providerName, c.instanceName)
	if err != nil {
		result.Healthy = false
		result.Error = err
		return result
	}

	start := time.Now()
	err = client.HealthCheck(ctx)
	result.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		result.Healthy = false
		result.Error = err
	}

	return result
}

// HealthCheckAllInstances iterates the instances sync.Map directly without
// triggering any Get/Build. For each cached instance, it invokes the
// instance's HealthCheck(ctx) and records the result. This is the safe
// variant to expose to callers (e.g. k8s liveness probes) that want to
// monitor the registry without accidentally constructing new clients.
//
// The returned slice is empty (not nil) when no instances have been built.
// Each result carries provider name, instance name, healthy boolean,
// latency in milliseconds, and the underlying error (or nil).
func HealthCheckAllInstances(ctx context.Context) []HealthResult {
	results := make([]HealthResult, 0)
	r := Registry()
	r.instances.Range(func(key, value any) bool {
		// key format: "providerName/instanceName"
		keyStr, ok := key.(string)
		if !ok {
			return true // skip malformed entries
		}
		parts := strings.SplitN(keyStr, "/", 2)
		if len(parts) != 2 {
			return true
		}
		providerName, instanceName := parts[0], parts[1]

		client, ok := value.(Client)
		if !ok {
			results = append(results, HealthResult{
				Provider: providerName,
				Instance: instanceName,
				Healthy:  false,
				Error:    fmt.Errorf("instance value is not a Client: %T", value),
			})
			return true
		}

		result := HealthResult{
			Provider: providerName,
			Instance: instanceName,
			Healthy:  true,
		}

		start := time.Now()
		err := client.HealthCheck(ctx)
		result.LatencyMs = time.Since(start).Milliseconds()
		if err != nil {
			result.Healthy = false
			result.Error = err
		}
		results = append(results, result)
		return true
	})
	return results
}
