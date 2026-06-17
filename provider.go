package registry

import "context"

// ClientProvider 客户端提供者接口（各中间件实现）
type ClientProvider interface {
	Name() string // e.g. "redis", "kafka"
	Build(instanceName string, cfg map[string]any) (Client, error)
	HealthCheck(ctx context.Context) error
	Close() error
}
