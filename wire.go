//go:build wireinject

package registry

import (
	"context"

	"github.com/google/wire"

	configx "github.com/lego2/configx/hubx"
)

// ProviderSet 提供给 Wire 的依赖集合
var ProviderSet = wire.NewSet(
	Registry,
	wire.Bind(new(ConfigLoader), new(*configx.ViperLoader)),
)

// NewViperLoaderFromPath 从路径创建 ViperLoader（Wire 入口）
func NewViperLoaderFromPath(path string) (*configx.ViperLoader, error) {
	return configx.NewViperLoader(path)
}

// InitializeApp 典型的应用初始化函数
func InitializeApp(ctx context.Context, coreInstances []string) error {
	return InitCore(ctx, coreInstances)
}
