# Contest Dispatch Kafka

## 功能概览
该模块用于在高并发场景下替代 Submit → Contest RPC 的资格校验链路，通过 Kafka 实现异步分流。Submit 仅做基础校验后写入 `contest.submit.validate`，Contest Service 消费消息完成资格校验，校验通过再转发到判题队列（`judge.level0`）。校验失败则由 Contest 直接写入最终状态（StatusFailed + error_code/error_message），避免 Submit 队列阻塞与 RPC 吞吐瓶颈。该链路与 RPC 共存，可在配置中心动态切换。

## 关键接口与数据结构
- **分流开关**：`submit.switch`（JSON：`{"mode":"rpc|kafka"}`）
- **消息结构**：复用 `services/submit_service/internal/domain.JudgeMessage`
  - 必填字段：`submission_id`、`contest_id`、`problem_id`、`user_id`、`created_at`
- **最终状态写入**：使用 `pkg/submit/statuswriter` 直接写 DB + Redis（摘要缓存）

## 使用示例与配置说明
Submit 配置（示例）：
- `Submit.ContestDispatch.topic: contest.submit.validate`
- `bootstrap.keys.switch: submit.switch`

Contest 配置（示例）：
- `ContestDispatch.topic: contest.submit.validate`
- `ContestDispatch.deadLetterTopic: contest.submit.validate.dead`
- `ContestDispatch.idempotencyTTL: 30m`
- `ContestDispatch.statusTTL: 24h`

切换策略：
1) 压力大时将 `submit.switch` 设置为 `{"mode":"kafka"}`
2) 压力小且追求低延迟时设置为 `{"mode":"rpc"}`
3) 切换无需重启，Submit 会订阅配置中心动态更新
