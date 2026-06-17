package registry

// ConfigLoader 配置加载器接口.
//
// @Deprecated: ConfigLoader is the legacy single-loader interface for
// the hubx registry. It supports a single global config source shared
// by every provider, has no per-provider (provider, instance) tagging
// on the inner map (the legacy envelope was a bare config map, not the
// `{config: <map>}` wrapper hubx drivers expect), and predates the
// configx Loader contract.
//
// New code MUST obtain a configx.Loader via
//
//	registry.Register(configx.NewProvider(configx.Config{Backend: "viper", Path: "..."}))
//	loader, err := registry.GetTyped[configx.Loader]("configx", "default")
//
// instead of calling registry.SetConfigLoader / registry.GetConfigLoader.
// SetConfigLoader / GetConfigLoader are kept as thin shims during the
// migration window and will be removed in a future major release.
type ConfigLoader interface {
	Load(providerName, instanceName string) (map[string]any, error)
	Watch(callback func(provider, instance string))
	Close() error
}
