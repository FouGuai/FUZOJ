# 判题服务（Judge Service）

## 功能概览
判题服务已迁移至 go-zero 框架，代码位于 `services/judge_service/`。入口为 `services/judge_service/judge.go`，配置为 `services/judge_service/etc/judge.yaml`。服务负责从 Kafka 拉取判题请求、拉取题目数据包并进行本地缓存、调用沙箱 Worker 执行判题，并将状态机写入 Redis 供前端轮询查询。服务关注高并发场景下的吞吐与稳定性，通过 Worker Pool 限流、Kafka 至少一次消费语义、以及本地 LRU+TTL 数据包缓存来降低存储与网络压力。判题流程中，题目元信息通过 ProblemService gRPC 获取，数据包通过 MinIO SDK 拉取并校验哈希，保证数据一致性。状态机遵循 Pending → Compiling(可选) → Running → Judging → Finished/Failed，失败时保留错误码与错误信息，便于重试与排查。

## 关键接口与数据结构
- go-zero 分层：`internal/handler`（HTTP 入口）→ `internal/logic`（业务编排）→ `internal/repository`（数据访问）→ `internal/model`（goctl 生成模型），依赖由 `internal/svc` 注入。
- Kafka 消息（JSON）：`submission_id`、`problem_id`、`language_id`、`source_key` 等字段。
- 状态查询：`GET /api/v1/judge/submissions/{id}`，返回判题状态、汇总与测试点结果。
- 本地缓存：以 `{problemId}/{version}` 目录组织，保存 `manifest.json`、`config.json` 与数据文件，并维护 `meta.json` 记录哈希。

## 使用说明
1) Judge Service 启动后订阅 Kafka 主题，按配置并发处理判题请求。
2) 通过 `ProblemService` gRPC 拉取最新元信息，若本地缓存未命中则从 MinIO 下载数据包并解压。
3) 下载源码到本地工作目录，构造 `JudgeRequest` 交给 Worker 执行。
4) 结果写入 Redis 状态机，前端通过轮询接口获取实时进度与最终结果。
