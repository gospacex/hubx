# hubx - 企业中间件注册表

> **一个统一的懒加载单例注册表，用于 `x` 系列SDK家族中的 50+ 第一方中间件客户端。**

[![Go 版本](https://img.shields.io/badge/go-1.21-blue)]()
[![状态](https://img.shields.io/badge/status-stable-green)]()

hubx 是 lego2 平台的核心中间件注册表，它抽象了 Redis、MySQL、Kafka、Elasticsearch、Jaeger、OpenTelemetry、MinIO、Consul 等基础设施客户端的构建、查找、健康检查和关闭过程，提供统一的通用 Go API。

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

## 快速入门

```go
package main

import (
    "context"
    "fmt"
    "github.com/gospacex/cachex/hubx/redisx"
    "github.com/lego2/hubx"
)

func init() {
    hubx.Register(redisx.New())
}

func main() {
    cli, err := hubx.Get("cachex.redis", "default")
    if err != nil { panic(err) }
    fmt.Println(cli.HealthCheck(context.Background()))
    defer hubx.Shutdown(context.Background())
}
```

## 主要功能

- **懒加载**：首次访问时构建客户端
- **缓存**：进程内单例
- **配置加载**：支持 Viper、Consul、Vault 等
- **健康检查**：无副作用批量探测
- **优雅关闭**：带钩子的关闭流程

## 架构

```
┌─────────────────────────────────────┐
│           业务代码                │
│   redis, _ := hubx.Get(...)       │
└─────────────────────┬──────────────┘
                        ▼
┌─────────────────────────────────────┐
│           hubx.Registry             │
│   providers map[string]ClientProvider│
│   instances sync.Map               │
└─────────────────────┬──────────────┘
                        ▼
┌─────────────────────────────────────┐
│         ClientProvider              │
│   (Redis、MySQL、Kafka 等驱动)      │
└─────────────────────────────────────┘
```

## 示例

- **QuickStart**：基础用法示例
- **CachexWireJaeger**：Redis + Jaeger 的完整示例
- **PlatformBase**：5 个驱动的完整 E2E 示例

## 安装

```bash
go get github.com/lego2/hubx@latest
```

## 文档

- [English](README.md) - 详细文档和 API 参考
- [中文](README.zh.md) - 中文文档和示例

## 许可证

MIT 许可证 - 查看 LICENSE 文件了解详情

---

**维护者**: @lego2/platform | **版本**: v1.0.0

[English](README.md) | [中文](README.zh.md)
