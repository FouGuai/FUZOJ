# Status Service

## 功能概览
Status Service 负责对外提供提交状态查询能力，是前端轮询判题进度的统一入口。服务以数据库中的 final_status 作为最终状态来源，Redis 作为加速缓存，遵循 Cache-Aside 模式，并对缓存雪崩、穿透与击穿做了防护（TTL 抖动、空值缓存、singleflight 合并）。当判题处于进行中时，服务会优先从 Redis 中读取判题服务写入的中间状态；当缓存缺失或已完成时，从数据库读取最终状态并回填缓存。状态详情支持按需返回，默认仅返回摘要字段，避免大字段对网络与缓存的压力。

## 关键接口或数据结构
- `GET /api/v1/status/submissions/:id`：查询单个提交状态
  - Query：`include`（可选，`details`/`log`）
  - 响应体：`JudgeStatusResponse`
- `JudgeStatusData`：提交状态数据结构（包含 verdict、summary、progress、timestamps 等）
- `StatusRepository`：统一封装 Redis + DB 查询与缓存回写逻辑
- `SubmissionLogRepository`：读取编译/运行日志（从 DB 或 MinIO 回源）

## 使用示例或配置说明
- 默认查询：`GET /api/v1/status/submissions/:id`
  - 返回摘要字段（不含 compile/tests）
- 详情查询：`GET /api/v1/status/submissions/:id?include=details`
  - 返回 compile/tests，并尝试从日志仓库补全日志内容
- 配置建议：
  - `status.cacheTTL=30m`、`status.cacheEmptyTTL=5m`
  - `status.logMaxBytes=65536` 用于控制日志 inline 大小
  - `status.timeouts.status=2s` 控制状态读取超时
