# zhi-id-gen-go

`zhi-id-gen-go` 是 `id-generator` 的 Go 重写版本，目标是与现有 Java 实现保持全兼容：

- HTTP API 路径与参数兼容
- `ApiResponse` 返回结构兼容
- PostgreSQL 表结构兼容
- `Snowflake` 与 `Segment` 两种模式兼容
- `DB Worker ID` 分配、续租、释放与备用切换兼容

## 当前采用的 Go 模板

仓库结构参考 `zhi-file-service-go` 的服务模板，并按单服务项目裁剪：

```text
zhi-id-gen-go/
├── cmd/id-generator-server
├── internal/platform
│   ├── bootstrap
│   ├── config
│   ├── httpserver
│   └── observability
├── internal/services/idgen
│   ├── app
│   ├── domain
│   ├── infra
│   ├── ports
│   ├── runtime
│   └── transport
├── pkg
│   ├── client
│   └── types
├── examples/go-client
├── sql
├── bootstrap
└── docs
```

## 当前状态

当前仓库已具备可运行首版：

- Segment 模式双缓冲缓存发号
- Segment 缓存观测接口 `/api/v1/id/cache/{bizTag}`
- Snowflake 静态 Worker 模式
- Snowflake DB Worker ID 抢占、续租、释放、备用切换
- Java 风格 `ApiResponse` 与主要错误码映射
- Go SDK 首版
- 兼容版 PostgreSQL schema 与基础单元测试

## 本地运行

```bash
go run ./cmd/id-generator-server
```

默认监听 `:8088`。

当前已提供：

- `GET /health`
- `GET /api/v1/id/health`
- `GET /api/v1/id/segment/{bizTag}`
- `GET /api/v1/id/segment/{bizTag}/batch?count=10`
- `GET /api/v1/id/tags`
- `GET /api/v1/id/cache/{bizTag}`
- `GET /api/v1/id/snowflake`
- `GET /api/v1/id/snowflake/batch?count=10`
- `GET /api/v1/id/snowflake/parse/{id}`
- `GET /api/v1/id/snowflake/info`

## 当前配置

当前运行需要：

- `DATABASE_URL`：当前服务启动必需，用于 Segment 和 DB Worker ID 模式

当前 Snowflake 支持：

- `WORKER_ID`
- `DATACENTER_ID`
- `SNOWFLAKE_EPOCH`
- `WORKER_ID_LEASE_TIMEOUT`
- `WORKER_ID_RENEW_INTERVAL`
- `BACKUP_WORKER_ID_COUNT`

说明：

- `WORKER_ID >= 0` 时走静态 Worker 模式
- `WORKER_ID = -1` 时走 `worker_id_alloc` 自动抢占模式
- 当前已支持主用 Worker ID 续租、备用 Worker ID 预分配与时钟回拨切换

## Go SDK

默认地址与服务默认端口一致，指向 `http://localhost:8088`。

```go
package main

import (
    "fmt"

    "github.com/architectcgz/zhi-id-gen-go/pkg/client"
)

func main() {
    c := client.New(client.DefaultConfig())
    defer c.Close()

    snowflakeID, _ := c.NextSnowflakeID()
    orderID, _ := c.NextSegmentID("order")

    fmt.Println(snowflakeID, orderID)
}
```

如需本地缓冲：

```go
c := client.New(client.Config{
    ServerURL:       "http://localhost:8088",
    BufferEnabled:   true,
    BufferSize:      100,
    RefillThreshold: 20,
    BatchFetchSize:  50,
    AsyncRefill:     true,
})
```

完整示例见 [examples/go-client/main.go](./examples/go-client/main.go)。
