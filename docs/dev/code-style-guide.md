# zhi-id-gen-go Code Style Guide

`zhi-id-gen-go` 采用从 `zhi-file-service-go` 抽取并裁剪后的 Go 服务模板。

## 目标

- 保持单服务项目也有清晰边界
- 让 `Snowflake`、`Segment`、`SDK` 后续演进不需要返工目录
- 明确平台能力与业务能力分离

## 目录模板

```text
cmd/id-generator-server

internal/platform/
  bootstrap/
  config/
  httpserver/
  observability/

internal/services/idgen/
  domain/
  ports/
  app/
    commands/
    queries/
    view/
  infra/
    postgres/
  transport/http/
  runtime/

pkg/
  client/
  types/
```

## 规则

1. `platform` 只放通用启动、配置、HTTP 服务和观测能力。
2. `services/idgen` 承载 ID 生成业务，不把业务规则放回 `platform`。
3. `transport/http` 只做协议适配和响应映射。
4. `app` 做用例编排；`domain` 放算法规则和不变量；`infra` 放 PostgreSQL 实现。
5. 对外稳定结构放 `pkg`，例如 Go SDK 和公共响应对象。

## 当前裁剪

当前仓库是单服务项目，所以没有照搬多服务模板里的全部平台依赖，只保留：

- `bootstrap`
- `config`
- `httpserver`
- `observability`

后续如需数据库、定时续租、指标和 tracing，再继续补到 `internal/platform/`。

