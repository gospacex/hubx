package registry

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// registry 内部类型，避免外部直接实例化
type registry struct {
	providers    map[string]ClientProvider
	instances    sync.Map // key: "providerName/instanceName"
	mu           sync.Mutex
	configLoader ConfigLoader
	closed       atomic.Bool
}

// 全局 registry 实例
var defaultRegistry *registry

// Registry 获取全局 registry 实例
func Registry() *registry {
	if defaultRegistry == nil {
		defaultRegistry = &registry{
			providers: make(map[string]ClientProvider),
		}
	}
	return defaultRegistry
}

// Reset 重置 registry（仅用于测试）
func Reset() {
	defaultRegistry = &registry{
		providers: make(map[string]ClientProvider),
	}
}

// Register 注册一个 Provider
func Register(provider ClientProvider) {
	if defaultRegistry == nil {
		defaultRegistry = &registry{
			providers: make(map[string]ClientProvider),
		}
	}
	defaultRegistry.mu.Lock()
	defer defaultRegistry.mu.Unlock()
	defaultRegistry.providers[provider.Name()] = provider
}

// Get 获取单例实例（懒加载）
func Get(providerName, instanceName string) (Client, error) {
	return Registry().Get(providerName, instanceName)
}

// GetTyped 泛型获取（编译期类型安全）
func GetTyped[T Client](providerName, instanceName string) (T, error) {
	var zero T
	client, err := Registry().Get(providerName, instanceName)
	if err != nil {
		return zero, err
	}
	return client.(T), nil
}

// SetConfigLoader installs the given loader as the active ConfigLoader for
// the registry. If a previous loader is already installed, it is closed
// first so that any background goroutines (e.g. a viper file-watcher) are
// released and the old loader is not silently leaked.
//
// @Deprecated: SetConfigLoader is the legacy single-loader API. New code
// MUST register a configx.Loader via
//
//	registry.Register(configx.NewProvider(configx.Config{Backend: "viper", Path: "..."}))
//	loader, err := registry.GetTyped[configx.Loader]("configx", "default")
//
// and stop calling SetConfigLoader. SetConfigLoader will be removed in a
// future major release; the method is retained as a thin shim during the
// migration window.
//
// @Deprecated: SetConfigLoader is the only public way to install a
// hubx.ConfigLoader; it is being replaced by the typed registry pattern
// above. See the @Deprecated notice on the ConfigLoader interface itself
// (hubx/config.go) for the full migration rationale.
//
// @Deprecated: Callers MUST migrate to hubx.Register(configx.NewProvider(...))
// + hubx.GetTyped[configx.Loader](...) before the next major release.
func SetConfigLoader(loader ConfigLoader) {
	r := Registry()
	r.mu.Lock()
	prev := r.configLoader
	r.configLoader = loader
	r.mu.Unlock()
	if prev != nil {
		_ = prev.Close()
	}
}

func (r *registry) Get(providerName, instanceName string) (Client, error) {
	if r.closed.Load() {
		return nil, ErrRegistryClosed
	}

	key := providerName + "/" + instanceName

	// 首次访问懒加载
	if v, ok := r.instances.Load(key); ok {
		return v.(Client), nil
	}

	// 双检查 + 锁
	r.mu.Lock()
	defer r.mu.Unlock()
	if v, ok := r.instances.Load(key); ok {
		return v.(Client), nil
	}

	// 构建新实例
	provider, ok := r.providers[providerName]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, providerName)
	}

	cfg, err := r.loadConfig(providerName, instanceName)
	if err != nil {
		return nil, fmt.Errorf("%w: %s/%s - %v", ErrConfigInvalid, providerName, instanceName, err)
	}

	client, err := provider.Build(instanceName, cfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %s/%s - %v", ErrBuildFailed, providerName, instanceName, err)
	}

	r.instances.Store(key, client)
	return client, nil
}

func (r *registry) loadConfig(providerName, instanceName string) (map[string]any, error) {
	if r.configLoader == nil {
		return nil, nil
	}
	return r.configLoader.Load(providerName, instanceName)
}
