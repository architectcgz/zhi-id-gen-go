# ID 生成高风险场景记录

本文档记录 `zhi-id-gen-go` 在发号链路中最容易出现问题的场景、预期行为、当前验证结果，以及后续仍需关注的风险点。

适用范围：

- `Snowflake` 发号
- `Segment` 发号
- `DB Worker ID` 租约模式
- 运行时高可用与扩容行为

## 已验证场景

### 1. 时钟回拨

风险说明：

- 系统时间回退时，`Snowflake` 可能生成重复 ID，或者时间戳部分倒退导致排序异常。

预期行为：

- 核心生成器直接返回 `ClockBackwardsError`
- 若启用 `DB Worker ID` 且存在备用 worker，则服务切换到备用 worker 后继续发号
- 若没有备用 worker，则返回业务错误 `CLOCK_BACKWARDS`

已验证结果：

- 回拨 `5ms` 时，核心生成器正确返回 `ClockBackwardsError`
- 回拨且存在备用 worker 时，`SnowflakeService` 能自动切换 worker 并继续发号
- 回拨且无备用 worker 时，`SnowflakeService` 返回 `CLOCK_BACKWARDS`，并携带 `offset`

相关测试：

- `internal/services/idgen/domain/snowflake_test.go`
- `internal/services/idgen/app/commands/snowflake_service_test.go`

### 2. 同毫秒序列溢出

风险说明：

- `Snowflake` 序列位为 `12` 位，单毫秒最多可发 `4096` 个 ID
- 若同一毫秒内继续请求，可能出现重复、回绕覆盖，或错误的自旋逻辑

预期行为：

- 序列耗尽后等待到下一毫秒继续生成
- ID 必须保持严格递增且不重复

已验证结果：

- 序列溢出后会等待下一毫秒再发号
- 连续生成跨越溢出边界时，ID 严格递增，无重复

相关测试：

- `internal/services/idgen/domain/snowflake_test.go`

### 3. Worker 租约失效

风险说明：

- `DB Worker ID` 模式下，租约续期失败后若实例继续使用旧 worker 发号，可能与其他实例冲突

预期行为：

- 第一次续租失败允许短暂抖动
- 连续失败达到阈值后，将当前 worker 标记为无效
- 后续发号请求应直接失败，不再继续使用旧 worker

已验证结果：

- 连续两次续租失败后，`WorkerLeaseManager` 会将 worker 标记为无效
- `SnowflakeService` 在检测到无效 worker 时会快速失败，返回 `WORKER_ID_INVALID`

相关测试：

- `internal/services/idgen/app/commands/worker_lease_manager_test.go`
- `internal/services/idgen/app/commands/snowflake_service_test.go`

### 4. 时钟回拨且无备用 Worker

风险说明：

- 租约模式下，若主 worker 已不可安全使用，且没有可切换的备用 worker，服务容易进入错误重试或错误发号

预期行为：

- 直接失败
- 返回明确错误码，不能继续生成潜在重复 ID

已验证结果：

- 当前实现会返回 `CLOCK_BACKWARDS`
- 响应中包含 `offset`，便于排查时钟问题

相关测试：

- `internal/services/idgen/app/commands/snowflake_service_test.go`

### 5. Segment 当前号段耗尽且下一段未就绪

风险说明：

- `Segment` 模式在双 buffer 切换时，如果当前段耗尽、下一段未加载完成，容易出现越界、重复发号，或错误切换

预期行为：

- 当前段耗尽且 next 段不可用时，直接返回错误
- 不允许继续发出无效或重复 ID

已验证结果：

- 当前实现会返回 `SEGMENTS_NOT_READY`
- 不会在未准备好的情况下继续发号

相关测试：

- `internal/services/idgen/domain/segment_buffer_test.go`

### 6. Segment 缓存观测值偏差

风险说明：

- `currentSegment.value`、`idle`、段边界若定义不一致，运维观测和排障会出现误判

预期行为：

- 号段语义保持 `[maxId-step, maxId)`
- `value` 表示“下一个将发出的 ID”
- `idle = max - value`

已验证结果：

- 当前快照语义已与实现保持一致
- `cache` 接口观测值可用于判断段切换和剩余容量

相关测试：

- `internal/services/idgen/domain/segment_buffer_test.go`

### 7. 静态 Snowflake 无数据库启动

风险说明：

- 若服务必须强依赖数据库，即使只想使用静态 `Snowflake` 模式，也会因数据库故障整体不可用

预期行为：

- 当 `WORKER_ID >= 0` 时，服务允许在无 `DATABASE_URL` 的情况下启动
- `Snowflake` 接口可用
- `Segment` 能力返回未初始化状态

已验证结果：

- 当前静态 `Snowflake` 模式已支持无数据库降级启动
- `/api/v1/id/health` 返回 `DEGRADED`
- `/api/v1/id/snowflake` 可正常发号

相关测试：

- `internal/services/idgen/runtime/runtime_test.go`

### 8. 数据库波动期间健康检查误判

风险说明：

- 若 `health` 每次都依赖实时查库，数据库瞬时抖动会把服务健康状态放大为不可用

预期行为：

- `health` 优先反映本地可用状态
- 已初始化实例在数据库短时故障时，健康检查仍应可返回

已验证结果：

- 当前 `health/tags` 走本地缓存视图
- PostgreSQL 故障后，`/api/v1/id/health` 仍可返回 `UP`

相关测试：

- `internal/services/idgen/runtime/runtime_integration_test.go`

### 9. 运行中新增 BizTag 的扩容感知

风险说明：

- 新增 `leaf_alloc.biz_tag` 后，如果实例无法感知新 tag，需要重启才能生效，不利于在线扩容

预期行为：

- 服务启动时预热已有 tags
- 运行中通过后台刷新自动感知新 tag
- 新 tag 应进入缓存观测视图

已验证结果：

- 当前已支持启动预热和后台刷新
- 真实 PostgreSQL 集成测试已验证运行中新增 `bizTag` 可被实例感知

相关测试：

- `internal/services/idgen/runtime/runtime_integration_test.go`

### 10. 多实例共享数据库的唯一性

风险说明：

- 多实例同时运行时，如果 `worker_id_alloc` 抢占冲突或 `Segment` 发号区间重叠，会直接产生重复 ID

预期行为：

- 多实例 `DB Worker ID` 分配结果必须互不相同
- 多实例通过 `/api/v1/id/snowflake` 发号时，跨实例 ID 不重复
- 多实例通过 `/api/v1/id/segment/{bizTag}` 发号时，跨实例 ID 不重复

已验证结果：

- 3 个实例共享同一 PostgreSQL 时，`workerId` 分配无冲突
- 3 个实例通过 `/api/v1/id/snowflake` 共生成 `4500` 个 ID，无重复
- 3 个实例通过 `/api/v1/id/segment/order/batch` 共生成 `3600` 个 ID，无重复

相关测试：

- 临时多实例集成验证，基于 `go test -tags=integration ./internal/services/idgen/runtime`

## 本机验证数据

### 核心生成器吞吐

已验证：

- 核心 `SnowflakeGenerator` 在本机 1 秒内可生成 `4,096,366` 个不重复 ID

说明：

- 这是进程内纯发号能力
- 不包含 HTTP、JSON、网络开销

### HTTP 接口吞吐

已验证：

- `GET /api/v1/id/snowflake` 在本机 1 秒内返回 `88,460` 个不重复 ID

说明：

- 这是通过 HTTP 路由的端到端结果
- 包含路由、JSON 编解码和 HTTP client/server 开销

### HTTP Batch 接口吞吐

已验证：

- `GET /api/v1/id/snowflake/batch?count=100` 在本机 1 秒内返回 `3,774,100` 个不重复 ID
- `GET /api/v1/id/snowflake/batch?count=1000` 在本机 1 秒内返回 `4,080,000` 个不重复 ID

说明：

- 上述结果都通过真实 HTTP 路由统计
- `count=1000` 已非常接近单实例核心生成器理论上限
- 当前 batch 接口场景下未发现重复 ID

### 多实例唯一性验证数据

已验证：

- `3` 个实例共享同一 PostgreSQL
- `Snowflake` 接口总计 `4500` 个 ID，无重复
- `Segment` 接口总计 `3600` 个 ID，无重复

说明：

- 该结果重点验证跨实例唯一性，不是吞吐上限
- 使用真实 PostgreSQL、真实 runtime、多实例共享 `worker_id_alloc` 和 `leaf_alloc`

## 尚未完全覆盖的风险

以下场景已识别，但仍建议继续做专门验证：

- 多实例同时争抢 `worker_id_alloc` 的真实并发竞争
- 长时间运行下的系统时间漂移、NTP 调整、虚拟机宿主机时间跳变
- `Segment` 模式在高并发下的真实数据库瓶颈和切段抖动
- 进程重启后 worker 释放与重新获取的时序稳定性
- 多节点总吞吐测试与跨实例去重验证

## 建议的回归测试顺序

每次发号核心逻辑、租约逻辑或 runtime 逻辑变更后，建议按以下顺序回归：

1. `go test ./...`
2. `go test -tags=integration ./internal/services/idgen/runtime -count=1`
3. 边界专项：
   - 时钟回拨
   - 序列溢出
   - worker 租约失效
   - segment 耗尽与切换
4. 吞吐专项：
   - 核心生成器压测
   - `/api/v1/id/snowflake` HTTP 压测
