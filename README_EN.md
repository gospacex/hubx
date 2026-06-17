# hubx - Enterprise Middleware Registry

> **A unified, lazy-singleton registry for 50+ first-party middleware clients across the lego2 `x` module family.**

[![Go Version](https://img.shields.io/badge/go-1.21-blue)]()
[![Status](https://img.shields.io/badge/status-stable-green)]()

hubx is the central middleware registry of the lego2 platform. It abstracts the construction, lookup, health-checking, and shutdown of every infrastructure client your service needs — Redis, MySQL, Kafka, Elasticsearch, Jaeger, OpenTelemetry, MinIO, Consul, etc. — behind a single, generic Go API.

Business code requests a client by `(provider, instance)` and gets back a fully-configured, type-safe handle:

```go
redis, err := hubx.GetTyped[*redis.Client]("cachex.redis", "default")
```

## Quick Start

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

## Key Features

- **Lazy Loading**: Builds clients on first access (no boot-time graph)
- **Caching**: Caches as process-wide singletons
- **Config Decoding**: Decodes config from registered loaders (Viper, Vault, Consul, in-memory)
- **Health Checking**: Checks every live instance without triggering new builds
- **Graceful Shutdown**: Closes every instance exactly once on `Shutdown`, aggregating errors

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                         Business Code                            │
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
                               │  Concrete x-sdk driver packages:   │
                               │   cachex/hubx/redisx               │
                               │   dbx/hubx/mysqlx                  │
                               │   mqx/hubx/kafkax/producer         │
                               │   otelx/hubx/tracer                │
                               │   osx/hubx/minio                   │
                               │   ... 50+ total                    │
                               └────────────────────────────────────┘
```

## Examples

- **QuickStart**: 10-line runtime registry usage
- **CachexWireJaeger**: Wire-based DI (Redis + Jaeger)
- **PlatformBase**: Full E2E: 5 drivers, Viper, health probe
- **wire-features**: Wire features, no external services

## Installation

Add hubx to your service's `go.mod`:

```bash
go get github.com/lego2/hubx@latest
```

Add only the drivers you use (each is its own module — no transitive driver bloat):

```bash
go get github.com/gospacex/cachex/hubx/redisx@latest
go get github.com/gospacex/dbx/hubx/mysqlx@latest
go get github.com/gospacex/otelx/hubx/tracer@latest
# ...
```

## Documentation

- [English](README.md) - Detailed documentation and API reference
- [中文](README.zh.md) - Chinese documentation and examples

## License

MIT License - See LICENSE file for details

---

**Maintainers**: @lego2/platform | **Stability**: hubx v1 contract is frozen. New drivers welcome via PR.

[English](README.md) | [中文](README.zh.md)