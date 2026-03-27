# 性能证据说明

本文档用于回答两个问题：

1. `zhi-id-gen-go` 的发号能力如何证明
2. 如何让他人独立复现这些结果

## 证据原则

对外给证据时，不要只报一个吞吐数字，需要同时提供：

- 测试命令
- 测试场景
- 是否单实例 / 多实例
- 是否核心函数 / HTTP 接口 / Batch 接口
- 原始输出
- 去重结果

## 可复现工具

仓库提供了一个可直接运行的压测命令：

```bash
go run ./cmd/id-generator-bench -mode http
```

支持模式：

- `http`：压测 `/api/v1/id/snowflake`
- `batch`：压测 `/api/v1/id/snowflake/batch`

默认行为：

- 如果不传 `-url`，命令会在进程内启动一个本地测试服务
- 如果传入 `-url`，命令会直接压测目标服务

## 推荐命令

### 1. 单实例 HTTP 单发接口

```bash
go run ./cmd/id-generator-bench -mode http -duration 1s
```

### 2. 单实例 HTTP Batch 接口

```bash
go run ./cmd/id-generator-bench -mode batch -count 100 -duration 1s
go run ./cmd/id-generator-bench -mode batch -count 1000 -duration 1s
```

### 3. 指定目标服务压测

```bash
go run ./cmd/id-generator-bench -mode batch -count 1000 -duration 1s -url http://127.0.0.1:8088
```

## 建议对外展示的证据包

### A. 核心能力证据

- 核心生成器 1 秒可生成多少 ID
- 是否重复
- 对应测试命令和输出

### B. HTTP 端到端证据

- `/api/v1/id/snowflake`
- `/api/v1/id/snowflake/batch`
- 吞吐、去重、批量大小

### C. 多实例唯一性证据

- 多实例共享同一 PostgreSQL
- `workerId` 是否冲突
- `Snowflake` 和 `Segment` 是否跨实例重复

## 当前已记录的结果

详见：

- [ID 生成高风险场景记录](./id-generation-risk-scenarios.md)

## 注意事项

- 核心生成器吞吐不等于 HTTP 吞吐
- 单实例吞吐不等于集群总吞吐
- 本机结果受 CPU、Go 版本、内核、网络栈影响
- 对外声明时应附上测试环境与命令
