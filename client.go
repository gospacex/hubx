package registry

import "context"

// Client 客户端通用接口
type Client interface {
	HealthCheck(ctx context.Context) error
	Close() error
}
