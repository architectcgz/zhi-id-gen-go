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

当前仓库已完成：

- 独立 Git 仓库初始化
- 远程仓库 `origin` 绑定
- Go 模块初始化
- 基于服务模板裁剪的基础骨架
- 统一响应结构定义
- 兼容版数据库 schema 初始化
- Go 风格文档初稿
- Segment 双缓冲缓存发号
- Snowflake 静态 Worker + DB Worker ID 模式首版

后续会按以下顺序推进：

1. Segment 缓存观测与更完整兼容接口
2. Go SDK
3. Docker、示例、文档与兼容测试

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

- `DATABASE_URL`：Segment 模式需要 PostgreSQL

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
