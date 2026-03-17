# Submit Service

## 功能概览
Submit Service 面向前端提供判题入口，负责接收提交请求、持久化源码与元信息、上传源码到对象存储，并将判题任务投递到 Kafka。该服务以高吞吐、低延迟为目标，内置限流与幂等机制，支持前端轮询查询状态与历史源码查看，避免重复提交与缓存穿透。

## 关键接口与数据结构
核心接口包括提交与查询：
- `POST /api/v1/submissions`：创建提交并投递判题，返回 `submission_id` 与初始状态。
- `GET /api/v1/submissions/{id}`：查询单个判题状态。
- `POST /api/v1/submissions/batch_status`：批量查询状态，返回 `items` 与 `missing`。
- `GET /api/v1/submissions/{id}/source`：获取历史源码。

Kafka 消息使用 `services/submit_service/internal/domain.JudgeMessage`，根据 `scene` 路由到不同层级主题（`level0-3`），并携带判题所需的源码位置、语言与优先级信息。

Contest 提交流程支持 RPC / Kafka 双通道：
- **RPC 模式**：Submit 调用 Contest RPC 进行资格校验，通过后直接写入 judge topic。
- **Kafka 模式**：Submit 仅做基础校验后写入 `contest.submit.validate`，由 Contest 校验通过后再转发到 judge topic。

## 使用示例与配置说明
服务配置位于 `services/submit_service/etc/submit.yaml`，支持配置：
- 源码大小上限、幂等 TTL、批量查询上限
- 限流窗口与阈值
- Redis/MySQL/MinIO/Kafka 连接信息

Kafka 主题必须提前创建（本地可通过 `make start` 自动创建）。Submit Service 至少依赖以下主题：
- `judge.level0` / `judge.level1` / `judge.level2` / `judge.level3`：判题任务队列（按 `scene` 路由）。
- `judge.status.final`：判题最终状态回传。
- `judge.status.final.dead`：最终状态消费失败的死信队列。
- `contest.submit.validate`：Contest 资格校验消息队列（Kafka 模式）。

**Topic 对齐要求**：Submit Service 根据 `scene` 选择 `topics.level0-3`，Judge Service 必须订阅相同的 Topic 列表并配置对应权重。若 Judge 订阅的是 `judge.task.*` 而 Submit 写入 `judge.level*`，消息会被投递到无人消费的 Topic，判题状态会长时间停留在 `Pending/Running`。

注意：topic 创建脚本会从配置文件中读取 `Topics` / `Kafka` / `Submit` 等配置段（大小写均支持），如果配置字段缺失或命名不规范，会导致 topic 未创建，从而出现 `Unknown Topic Or Partition` 的提交失败。

动态切换配置示例：
- `submit.switch`：`{"mode":"rpc|kafka"}`（运行时生效）

源码上传后会在数据库中持久化，并写入 Redis 缓存（TTL 默认 30 分钟，空值缓存默认 5 分钟）。判题状态写入 Redis，前端通过轮询接口获取实时进度；批量查询会返回缺失的 submission_id 列表以降低重复查询成本。

为降低消息乱序与进程异常导致的“提交长期停留未完成”风险，Submit Service 增加了 `submission_dispatch_outbox` 调度表与后台超时回投任务：提交创建时写入待调度记录，若在超时窗口内未收到最终状态，将按场景重投到 Judge 或 Contest 校验队列；最终状态消费成功后会将对应调度记录标记为 done。
