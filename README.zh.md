# hubx — 企业中间件注册表和 DI 锚点

> **一个统一的懒加载单例注册表，用于 lego2 `x` 模块家族中的 50+ 第一方中间件客户端。**
>
> 两种注入路径 · 类型安全泛型 · 一次性生命周期 · 竞态检测干净。

[![Go 版本](https://img.shields.io/badge/go-%E2%89%A51.21-blue)]()
[![竞态](https://img.shields.io/badge/go%20test--%20race--green)]()
[![核心覆盖率](https://img.shields.io/badge/coverage%20core-%E2%89%A590%25-green)]()
[![驱动覆盖率](https://img.shields.io/badge/coverage%20providers-%E2%89%A580%25-green)]()
[![驱动数量](https://img.shields.io/badge/providers-50%2B-blue)]()

---

## 目录

1. [什么是 hubx？](#1-what-is-hubx)
2. [为什么需要 hubx？](#2-why-hubx)
3. [架构](#3-architecture)
4. [安装](#4-installation)
5. [快速入门](#5-quick-start)
6. [核心概念](#6-core-concepts)
7. [配置加载器](#7-configuration-loaders)
8. [生命周期管理](#8-lifecycle-management)
9. [错误模型](#9-error-model)
10. [可观测性与健康检查](#10-observability--health-checks)
11. [并发保证](#11-concurrency-guarantees)
12. [驱动目录](#12-provider-catalog)
13. [示例](#13-examples)
14. [测试策略](#14-testing-strategy)
15. [生产部署](#15-production-deployment)
16. [迁移指南](#16-migration-guide)
17. [性能](#17-performance)
18. [版本与兼容性](#18-versioning--compatibility)
19. [故障排除](#19-troubleshooting)
20. [许可证](#20-license)

---

## 1. 什么是 hubx？

`hubx` 是 lego2 平台的核心中间件注册表。它抽象了每个基础设施客户端（如 Redis、MySQL、Kafka、Elasticsearch、Jaeger、OpenTelemetry、MinIO、Consul 等）的构建、查找、健康检查和关闭过程，提供统一的通用 Go API。

业务代码通过 `(provider, instance)` 请求客户端，并获取一个完全配置的、类型安全的句柄：

```go
redis, err := hubx.GetTyped[*redis.Client]("cachex.redis", "default")
```

其内部工作原理如下：

- **懒加载**：首次访问时构建客户端（无启动时图）
- **缓存**：进程内单例缓存
- **解码**：从注册的加载器（Viper、Vault、Consul、in-memory）中解码配置
- **健康检查**：检查每个活跃实例，无需触发新构建
- **关闭**：在 `Shutdown` 时关闭每个实例一次，聚合错误

## 2. 为什么需要 hubx？

| 问题（无 hubx） | hubx 解决方案 |
|---|---|
| 每个模块都有自己的 `New(cfg)` 工厂 → 散布的 `must.MustInit(...)` 调用 | 一个全局的 `Get(provider, instance)` 查找 |
| 启动顺序是隐式的、脆弱的、重复的 | 懒加载 + 首次使用，无需启动顺序 |
| 健康检查是每个驱动的 ad-hoc | 一个 `HealthCheckAllInstances(ctx)` 适用于整个平台 |
| 优雅关闭需要每个团队记住每个 `defer Close()` | 一个 `hubx.Shutdown(ctx)` 遍历整个注册表 |
| Wire / FX / 手动注入 — 选一个 | **两种** 静态 DI 和运行时注册表都是第一级的 |
| 多租户 / 多实例 ("redis-primary"、"redis-replica") | 实例名在查找中是第一级的 |

## 3. 架构

```
┌──────────────────────────────────────────────────────────────────┐
│                         业务代码                            │
│   redis, _ := hubx.GetTyped[*redis.Client]("cachex.redis",      │
│                                            "primary")           │
└───────────────────────────────────┬──────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────┐
│                      hubx.Registry  (sync.Map)                   │
│                                                                  │
│   providers:  map[string]ClientProvider  (eager register)        │
│   instances:  sync.Map["prov/inst"] → Client  (lazy singleton)   │
│   closed:     atomic.Bool                                         │
└──────────┬───────────────────────────────────────┬───────────────┘
           │                                       │
           ▼                                       ▼
    ┌───────────────┐                       ┌───────────────┐
    │  ConfigLoader │                       │ ClientProvider│
    │  (Viper, etc.)│                       │  (per driver) │
    └───────────────┘                       └───────────────┘
                                                    │
                                                    ▼
                               ┌────────────────────────────────────┐
                               │  具体 x-sdk 驱动包:   │
                               │   cachex/hubx/redisx               │
                               │   dbx/hubx/mysqlx                  │
                               │   mqx/hubx/kafkax/producer         │
                               │   otelx/hubx/tracer                │
                               │   osx/hubx/minio                   │
                               │   ... 50+ total                    │
                               └────────────────────────────────────┘
```

### 模块布局

| 路径 | 用途 |
|---|---|
| `hubx/` | 核心注册表、生命周期、健康检查、错误、wire 粘合 |
| `hubx/examples/` | 端到端示例 (`CachexWireJaeger`、`PlatformBase` 等) |
| `<x-module>/hubx/<driver>/` | 每个驱动的 `ClientProvider` 实现 |

每个 `<x-module>` 都是一个独立的 Go 模块；hubx 合同是唯一需要的合同。这保持了小的 blast radius — 你可以升级一个驱动而无需重建整个世界。

## 4. 安装

在您的服务的 `go.mod` 中添加 hubx：

```bash
go get github.com/lego2/hubx@latest
```

只添加您使用的驱动（每个驱动都是独立的模块 — 没有驱动膨胀）：

```bash
go get github.com/gospacex/cachex/hubx/redisx@latest
go get github.com/gospacex/dbx/hubx/mysqlx@latest
go get github.com/gospacex/otelx/hubx/tracer@latest
# ...
```

> **替换指令** (在 lego2 上本地开发时使用):
> ```go
> // go.mod
> require github.com/lego2/hubx v0.0.0
> replace github.com/lego2/hubx => ../hubx
> ```

## 5. 快速入门

### 5.1 运行时注册表 (5 行)

```go
package main

import (
    "context"
    "fmt"

    "github.com/gospacex/cachex/hubx/redisx"
    "github.com/lego2/hubx"
)

func init() {
    hubx.Register(redisx.New())      // 启动时注册一次
}

func main() {
    cli, err := hubx.Get("cachex.redis", "default")
    if err != nil { panic(err) }
    fmt.Println(cli.HealthCheck(context.Background()))
    defer hubx.Shutdown(context.Background())
}
```

### 5.2 带配置加载器

```go
import (
    "github.com/gospacex/cachex/hubx/redisx"
    configx "github.com/lego2/configx/hubx"
    "github.com/lego2/hubx"
)

func init() {
    loader, _ := configx.NewViperLoader("/etc/myapp/config.yaml")
    hubx.SetConfigLoader(loader)
    hubx.Register(redisx.New())
}
```

`config.yaml`:

```yaml
providers:
  cachex.redis.default:
    config:
      addr: redis-primary.prod:6379
      db: 0
  cachex.redis.replica:
    config:
      addr: redis-replica.prod:6379
      db: 0
```

```go
primary, _ := hubx.Get("cachex.redis", "default")
replica, _ := hubx.Get("cachex.redis", "replica")
```

### 5.3 使用 Wire (编译时 DI)

```go
// wire.go
//go:build wireinject
package main

import (
    "github.com/google/wire"

    "github.com/gospacex/cachex/hubx/redisx"
    "github.com/lego2/hubx"
    "github.com/gospacex/otelx/hubx/tracer"
)

func initializeApp() (*App, error) {
    wire.Build(
        NewApp,
        redisx.NewSet,        // wire.ProviderSet
        tracer.NewSet,
    )
    return nil, nil
}
```

两种路径都是 **第一级的** — 选择适合您团队的。

## 6. 核心概念

### 6.1 `ClientProvider` 接口

```go
type ClientProvider interface {
    Name() string                                              // "cachex.redis"
    Build(instanceName string, cfg map[string]any) (Client, error)
    HealthCheck(ctx context.Context) error
    Close() error
}
```

| 方法 | 调用时机 | 必须 |
|---|---|---|
| `Name()` | 在 `Register` 时 | 纯函数，返回标准名称 |
| `Build(...)` | 首次 `Get(p, i)` 时 | 线程安全，每个调用都是幂等的（注册表强制单例） |
| `HealthCheck(ctx)` | 不由核心调用；仅暴露给驱动级别的自省 | 廉价，无副作用 |
| `Close()` | 在 `hubx.Shutdown` 时 | 幂等的，返回聚合错误 |

### 6.2 `Client` 接口

```go
type Client interface {
    HealthCheck(ctx context.Context) error
    Close() error
}
```

`Client` 是驱动返回的任何东西 — `*redis.Client`、`*sql.DB`、`*kafka.Producer` 等。注册表将其存储为 `any`，`GetTyped[T]` 在调用站点执行类型断言。

### 6.3 注册表

一个单一的、懒加载的、线程安全的容器，持有：

- `providers map[string]ClientProvider` — 急切的，由 `Register` 填充
- `instances sync.Map` — 懒加载的，由 `Get` 填充。键是 `"providerName/instanceName"`
- `closed atomic.Bool` — 在 `Shutdown` 后翻转到 `true`；后续的 `Get` 调用返回 `ErrRegistryClosed`

### 6.4 实例标识

每个实例由 `(providerName, instanceName)` 标识：

```
"cachex.redis" + "default"    → first replica
"cachex.redis" + "replica"    → read-only replica
"dbx.mysql"   + "billing"     → MySQL for billing service
"otel.tracer" + "prod"        → production tracer
```

相同提供商，不同实例，完全不同的配置。

## 7. 配置加载器

`ConfigLoader` 回答的问题是 *"对于这个实例，我应该把什么原始配置地图交给 `provider.Build`？"*。默认加载器返回 `nil`（驱动使用零值配置）。可插拔的加载器：

### 7.1 Viper 加载器 (随 configx 提供)

```go
loader := viperhubx.New("config.yaml")   // 或 NewFromViper(v)
hubx.SetConfigLoader(loader)
```

支持热重载，通过 `loader.Watch(callback)` — 当底层的 Viper 实例检测到配置变化时，回调会触发。

### 7.2 自定义加载器 (例如来自 Consul / Vault)

```go
type Loader struct{ client *consul.Client }

func (l *Loader) Load(provider, instance string) (map[string]any, error) {
    raw, _, err := l.client.KV().Get(fmt.Sprintf("%s/%s", provider, instance), nil)
    if err != nil { return nil, err }
    return yaml.Unmarshal(raw.Value)
}
```

注意：`hubx` **不**自动监视自定义加载器。对于热重载，在应用代码中订阅，在回调中重新 `Get`。

## 8. 生命周期管理

### 8.1 首次使用生命周期

```
boot         ────  Register(p)  ────  (provider stored)
first call   ────  Get(p, i)     ────  loader.Load → p.Build → store
subsequent   ────  Get(p, i)     ────  return cached
shutdown     ────  Shutdown(ctx) ────  Close all + hooks
post-shutdown ───  Get(p, i)     ────  ErrRegistryClosed
```

### 8.2 关闭语义

`Shutdown(ctx)` 是 **幂等的和一次性** 的：

1. 如果注册表已经关闭 → 立即返回 `ErrRegistryClosed`
2. 否则翻转 `closed` 到 `true`（任何在途的 `Get` 调用在之后返回 `ErrRegistryClosed`）
3. 原子性地拉出已注册的关闭钩子并清空槽，因此第二次调用 `Shutdown` 不会重放旧钩子
4. 按反向注册顺序 (LIFO — 最后注册的先清理) 执行钩子
5. 关闭每个已注册的提供商（如果提供商本身是幂等的，则也是幂等的）
6. 使用 `errors.Join` 聚合所有错误，并 wrap `ErrShutdownFailed`。
   使用 `errors.Is(err, hubx.ErrShutdownFailed)` 检测部分失败。

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := hubx.Shutdown(ctx); err != nil {
    if errors.Is(err, hubx.ErrShutdownFailed) {
        log.Error("partial shutdown failure", "err", err)
    }
}
```

### 8.3 关闭钩子

对于不是提供商的东西（自定义日志记录器、临时文件清理器等）：

```go
hubx.RegisterShutdownHook("flush-audit-log", func(ctx context.Context) error {
    return audit.Flush(ctx)
})
```

钩子在 **提供商关闭之前** 触发 — 因此钩子仍然可以安全地 `Get` 一个客户端（如果需要的话）。

## 9. 错误模型

所有哨兵错误都是包级别的变量，可以通过 `errors.Is` 连接：

| 哨兵 | 含义 | 何时 |
|---|---|---|
| `ErrProviderNotFound` | 没有在注册表中找到提供商 | `Get` 未知名称 |
| `ErrInstanceNotFound` | (保留) | n/a 在当前实现中 |
| `ErrConfigInvalid` | ConfigLoader 返回了错误的配置 | `Get` 触发 `loadConfig` |
| `ErrBuildFailed` | 驱动的 `Build` 返回了错误 | `Get` 触发 `provider.Build` |
| `ErrRegistryClosed` | 注册表已经被关闭 | 任何 `Get` 调用在 `Shutdown` 之后 |
| `ErrShutdownFailed` | 至少一个提供商的 `Close` 失败 | wrap 目标在 `Shutdown` 错误中 |

包装模式 (匹配 Go 约定)：

```go
return fmt.Errorf("redis read: %w", hubx.ErrBuildFailed)
```

检查：

```go
if errors.Is(err, hubx.ErrBuildFailed) { ... }
```

## 10. 可观测性与健康检查

### 10.1 每个实例的检查

```go
cli, _ := hubx.Get("dbx.mysql", "billing")
if err := cli.HealthCheck(ctx); err != nil {
    log.Error("mysql unhealthy", "err", err)
}
```

### 10.2 批量探测 (无 Build 副作用)

```go
for _, r := range hubx.HealthCheckAllInstances(ctx) {
    fmt.Printf("%s/%s healthy=%v latency=%dms err=%v\n",
        r.Provider, r.Instance, r.Healthy, r.LatencyMs, r.Error)
}
```

这直接遍历 `sync.Map`。**它不会触发 `Build`** 用于未加载的实例 — 安全地从 `/healthz` 端点调用，而不会意外地构建客户端。

### 10.3 Kubernetes 探针接线

```go
// liveness.go
http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
    results := hubx.HealthCheckAllInstances(r.Context())
    for _, res := range results {
        if !res.Healthy {
            w.WriteHeader(http.StatusServiceUnavailable)
            return
        }
    }
    w.WriteHeader(http.StatusOK)
})
```

### 10.4 指标

`hubx` **不**发射自己的指标 — 在调用站点 wrap `Get` 和 `Build`，或在驱动上进行指标仪表化。`HealthResult` 中的 `LatencyMs` 字段足以实现一个简单的 RED 仪表板。

## 11. 并发保证

| 操作 | 安全? | 说明 |
|---|---|---|
| `Register` 来自多个 goroutine | ✅ | 互斥 map |
| `Get` 并发与 `Get` | ✅ | sync.Map + 互斥 double-check |
| `Get` 竞态与 `Build` | ✅ | First wins, others get the same instance |
| `Shutdown` 竞态与 `Get` | ⚠️ | First post-shutdown `Get` returns `ErrRegistryClosed`；the first `Shutdown` does the work |
| `Shutdown` called twice | ✅ | Second returns `ErrRegistryClosed` |
| `RegisterShutdownHook` racing with `Shutdown` | ⚠️ | Add hooks **before** shutdown; late hooks after the one-shot swap will not fire |

`go test -race ./...` is clean across the entire `hubx` tree.

## 12. 驱动目录

| x 模块 | 提供商名称 | 包 |
|---|---|---|
| **cachex** | cachex.redis | `cachex/hubx/redisx` |
| | cachex.memcache | `cachex/hubx/memcachex` |
| | cachex.bigcache | `cachex/hubx/bigcachex` |
| | cachex.freecache | `cachex/hubx/freecachex` |
| | cachex.inmemory | `cachex/hubx/inmemoryx` |
| | cachex.ristretto | `cachex/hubx/ristrettox` |
| | cachex.syncmap | `cachex/hubx/syncmapx` |
| | cachex.fake | `cachex/hubx/fakex` |
| **colx** | colx.cassandra | `colx/hubx/cassandrax` |
| | colx.clickhouse | `colx/hubx/clickhousex` |
| | colx.druid | `colx/hubx/druidx` |
| | colx.hbase | `colx/hubx/hbasex` |
| | colx.vertica | `colx/hubx/verticax` |
| **configx** | configx.viper | `configx/hubx` (loader) |
| **coordx** | coordx.consul | `coordx/hubx/consulx` |
| | coordx.etcd | `coordx/hubx/etcdx` |
| | coordx.nacos | `coordx/hubx/nacosx` |
| | coordx.regdis | `coordx/hubx/regdisx` |
| **dbx** | dbx.dm | `dbx/hubx/dmx` |
| | dbx.kingbase | `dbx/hubx/kingbasex` |
| | dbx.mysql | `dbx/hubx/mysqlx` |
| | dbx.oracle | `dbx/hubx/oraclex` |
| | dbx.postgres | `dbx/hubx/postgresx` |
| | dbx.sqlite | `dbx/hubx/sqlitex` |
| | dbx.tidb | `dbx/hubx/tidbx` |
| **docx** | docx.couchbase | `docx/hubx/couchbasex` |
| | docx.mongo | `docx/hubx/mongox` |
| **graphx** | graphx.age | `graphx/hubx/agex` |
| | graphx.dgraph | `graphx/hubx/dgraphx` |
| | graphx.janusgraph | `graphx/hubx/janusgraphx` |
| | graphx.memgraph | `graphx/hubx/memgraphx` |
| | graphx.nebula | `graphx/hubx/nebulax` |
| | graphx.neo4j | `graphx/hubx/neo4jx` |
| | graphx.tigergraph | `graphx/hubx/tigergraphx` |
| | graphx.tugraph | `graphx/hubx/tugraphx` |
| **lockerx** | lockerx.etcd | `lockerx/hubx/etcdx` |
| | lockerx.redis | `lockerx/hubx/redisx` |
| **mqx** | mqx.kafka.producer / consumer | `mqx/hubx/kafkax/{producer,consumer}` |
| | mqx.mqtt.publisher / subscriber | `mqx/hubx/mqttx/{publisher,subscriber}` |
| | mqx.nats.publisher / subscriber | `mqx/hubx/natsx/{publisher,subscriber}` |
| | mqx.nsq.producer / consumer | `mqx/hubx/nsqx/{producer,consumer}` |
| | mqx.pulsar.producer / consumer | `mqx/hubx/pulsarx/{producer,consumer}` |
| | mqx.rabbit.publisher / consumer | `mqx/hubx/rabbitx/{publisher,consumer}` |
| | mqx.redis-stream.producer / consumer | `mqx/hubx/redisstreamx/{producer,consumer}` |
| | mqx.rocketmq.producer / consumer | `mqx/hubx/rocketmqx/{producer,consumer}` |
| **osx** | osx.aws-s3 / aws-s3-object / aws-s3-path | `osx/hubx/awss3x` etc. |
| | osx.fake / huawei-obs / minio / oss / rustfs / tencent-cos | `osx/hubx/<driver>x` |
| **otelx** | otel.tracer | `otelx/hubx/tracer` |
| | otel.meter | `otelx/hubx/meter` |
| | otel.log | `otelx/hubx/log` |
| **searchx** | searchx.elasticsearch / manticore / meilisearch / solr / sphinx / typesense / vespa | `searchx/hubx/<driver>x` |
| **vectorx** | vectorx.milvus / qdrant / weaviate | `vectorx/hubx/<driver>x` |

> **mqx split**: producer 和 consumer 是独立的 `ClientProvider`，因为它们有不同的连接池、生命周期和资源计数。注册两个具有不同实例名的。

## 13. 示例

| 示例 | 演示 |
|---|---|
| [`examples/QuickStart`](examples/QuickStart) | 10 行运行时注册表用法 |
| [`examples/CachexWireJaeger`](examples/CachexWireJaeger) | Wire 基于的 DI (Redis + Jaeger) |
| [`examples/PlatformBase`](examples/PlatformBase) | 完整 E2E：5 个驱动、Viper、健康探针 |
| [`examples/wire-features`](examples/wire-features) | Wire 功能，无外部服务 |

```bash
cd examples/PlatformBase
make up    # docker-compose up -d
make test  # go test -race ./...
make down
```

## 14. 测试策略

每个包都带 **11 个单元测试 + 1 个集成测试**：

1. `TestName_ReturnsCorrectString`
2. `TestBuild_Success`
3. `TestBuild_MissingConfigKey` → `ErrConfigInvalid`
4. `TestBuild_MissingRequiredField` → `ErrConfigInvalid`
5. `TestBuild_UnknownField` → `ErrConfigInvalid`
6. `TestBuild_DriverNewFailure` → `ErrBuildFailed`
7. `TestProviderHealthCheck_NoOp`
8. `TestProviderClose_NoOp`
9. `TestClientHealthCheck`
10. `TestConcurrentBuild_Singleton`
11. `TestRaceFree_UnderRace`

集成测试被限制在 `//go:build integration` 后，除非相应的环境变量（例如 `REDIS_ADDR`）被设置：

```bash
go test -race -short ./...                                    # unit
REDIS_ADDR=localhost:6379 go test -race -tags=integration ./...  # full
```

覆盖率目标：

- `hubx/` 核心: **≥ 90 %**
- 每个 `<x>/hubx/<driver>/`: **≥ 80 %**

## 15. 生产部署

### 15.1 优雅关闭序列

```go
ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer cancel()

// 1. 停止接受新流量
httpSrv.Shutdown(ctx)

// 2. 耗尽后台工作者
workerWg.Wait()

// 3. 关闭每个中间件客户端
hubx.Shutdown(ctx)  // 幂等的；one-shot；30s 超时推荐
```

### 15.2 Pod 生命周期

```yaml
lifecycle:
  preStop:
    exec:
      command: ["sleep", "5"]   # let LB drain
  terminationGracePeriodSeconds: 60
```

`hubx.Shutdown` honors the supplied context — pass `context.WithTimeout(ctx, 30*time.Second)`。

### 15.3 多租户模式

```yaml
providers:
  cachex.redis.tenantA:
    config: { addr: redis-tenantA:6379, db: 0 }
  cachex.redis.tenantB:
    config: { addr: redis-tenantB:6379, db: 0 }
```

```go
cli, _ := hubx.Get("cachex.redis", tenantID)   // tenantID is the instance name
```

### 15.4 可观测性钩子

Wrap `hubx.Get` at the call site to emit metrics:

```go
func Get(provider, instance string) (hubx.Client, error) {
    start := time.Now()
    defer func() {
        metrics.ObserveGet(provider, instance, time.Since(start))
    }()
    return hubx_registry.Get(provider, instance)
}
```

## 16. 迁移指南

### 从每个驱动的 `must.MustInit`

```go
// before
redisCli := must.MustInit(redis.New(cfg))

// after
redisCli, _ := hubx.GetTyped[*redis.Client]("cachex.redis", "default")
```

### 从手卷工厂映射

```go
// before
var clients sync.Map
func GetRedis(name string) *redis.Client {
    if v, ok := clients.Load(name); ok { return v.(*redis.Client) }
    cli := redis.New(loadCfg(name))
    clients.Store(name, cli)
    return cli
}

// after
hubx.Register(redisx.New())
cli, _ := hubx.GetTyped[*redis.Client]("cachex.redis", name)
```

### 从 `fx` / `dig`

You can keep your DI container — hubx plugs into it via `wire.NewSet`
exports. Both styles coexist.

## 17. 性能

| 操作 | 成本 | 说明 |
|---|---|---|
| `Register` | ~ 50 ns | 一个 map 插入，带互斥 |
| `Get` (warm cache) | ~ 80 ns | sync.Map read |
| `Get` (cold, lazy build) | Depends on driver | One-time cost per process |
| `HealthCheckAllInstances` (N instances) | O(N) | All calls serial; each driver's `HealthCheck` should be O(1) |
| `Shutdown` | O(hooks + providers) | Errors joined, no retries |

Memory: one `Client` per `(provider, instance)` for the lifetime of the
process. Each provider package adds < 200 KB binary.

## 18. 版本与兼容性

- `hubx` follows [semver](https://semver.org/)。v1 合同
  (`ClientProvider`、`Client`、注册表函数、哨兵错误) 是 **冻结的**
  直到 v2。
- 每个驱动独立版本 — 升级一个而无需触及其他。
- 驱动 Config 字段类型的破坏性变化需要 **驱动包** 的主要版本 bump，而不是 hubx 本身。

## 19. 故障排除

| 症状 | 可能原因 | 修复 |
|---|---|---|
| `ErrProviderNotFound` | 忘记 `hubx.Register(driver.New())` | 在 `init()` 时注册 |
| `ErrConfigInvalid` | YAML 有未知键 | 检查错误中的 offending 键 |
| `ErrBuildFailed` | 驱动的 `New` 返回了错误 | 验证 DSN/端点；检查凭据 |
| `ErrRegistryClosed` | 在 `Shutdown` 之后调用 `Get` | 重构：shutdown 是终端 |
| `ErrShutdownFailed` | 一个提供商的 `Close` 失败 | 检查 joined 错误；partial failures are normal |
| Hot-reload doesn't pick up changes | Loader didn't fire `Watch` | Wire `loader.Watch(cb)` and re-`Get` in callback |

## 20. 许可证

内部 lego2 组件 — 请参阅仓库根部的许可证条款。

---

**维护者**: @lego2/platform
**稳定性**: `hubx` v1 合同是冻结的。新驱动欢迎通过 PR 贡献。

[English](README.md) | [中文](README.zh.md)