package registry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// shutdownHook 用户自定义关闭钩子
type shutdownHook struct {
	name string
	fn   func(ctx context.Context) error
}

var (
	shutdownHooks []shutdownHook
	hooksMu       sync.Mutex
)

// RegisterShutdownHook 注册关闭钩子
func RegisterShutdownHook(name string, hook func(ctx context.Context) error) {
	hooksMu.Lock()
	defer hooksMu.Unlock()
	shutdownHooks = append(shutdownHooks, shutdownHook{name: name, fn: hook})
}

// InitCore 启动时初始化核心实例
func InitCore(ctx context.Context, coreInstances []string) error {
	for _, instance := range coreInstances {
		parts := strings.Split(instance, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid core instance format: %s (expected provider:instance)", instance)
		}
		providerName, instanceName := parts[0], parts[1]

		_, err := Get(providerName, instanceName)
		if err != nil {
			return fmt.Errorf("init core %s/%s: %w", providerName, instanceName, err)
		}
	}

	return nil
}

// Shutdown 统一关闭所有实例。
//
// 当任一关闭钩子或 Provider.Close() 返回非 nil 错误,Shutdown 会把这些错误
// 通过 errors.Join 聚合成一个组合错误返回。组合错误同时 wrap ErrShutdownFailed,
// 调用方可使用 errors.Is(err, ErrShutdownFailed) 检测是否发生了关闭失败。
//
// 第二次调用 Shutdown 是幂等的:registry 已 closed 时,直接返回 ErrRegistryClosed。
func Shutdown(ctx context.Context) error {
	// 已关闭 → 幂等返回
	if Registry().closed.Load() {
		return ErrRegistryClosed
	}

	var errs []error

	// 1. 执行用户钩子(按注册顺序逆序)。
	//    取下 hook 列表后立即清空,保证 hooks 一次性:同一进程多次 Shutdown 不会重放旧 hook。
	hooksMu.Lock()
	hooks := shutdownHooks
	shutdownHooks = nil
	hooksMu.Unlock()

	for i := len(hooks) - 1; i >= 0; i-- {
		if err := hooks[i].fn(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown hook %s: %w", hooks[i].name, err))
		}
	}

	// 2. 关闭所有 Provider
	for name, provider := range Registry().providers {
		if err := provider.Close(); err != nil {
			errs = append(errs, fmt.Errorf("provider %s close: %w", name, err))
		}
	}

	// 3. 标记 registry 为已关闭
	Registry().closed.Store(true)

	// 4. 多错误聚合:用 errors.Join 合并所有 hook/provider 关闭错误。
	//    同时 wrap ErrShutdownFailed,便于 errors.Is 鉴别。
	if joined := errors.Join(errs...); joined != nil {
		return fmt.Errorf("shutdown: %d error(s): %w: %w", len(errs), ErrShutdownFailed, joined)
	}
	return nil
}
