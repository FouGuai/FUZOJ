# Status SSE Service

## 功能概览
Status SSE Service 提供提交状态的单向实时推送能力，用于替代高频轮询。客户端通过 SSE 连接订阅单个 `submission_id`，服务先发送当前快照，再在状态变化时推送增量更新。服务本身不写判题状态，只读取 Redis/DB 中已有状态并转发，保持无状态设计，支持多实例水平扩展。

## 关键接口与数据结构
- SSE：`GET /api/v1/status/submissions/:id/events`
  - Header：网关鉴权后透传 `X-User-Id`
  - Query：`include`（可选）
- 事件类型：`snapshot`、`update`、`final`
- Redis Pub/Sub：`submission:status:pubsub:{submission_id}`
- `StatusRepository`：负责归属校验、状态读取与终态详情回源

## 使用说明
1) 客户端建立 SSE 连接后，服务立即返回 `snapshot`。
2) Submit/Judge 写状态后会发布 Pub/Sub 事件，SSE 服务收到后按 `submission_id` 推送 `update`。
3) 当提交进入终态（Finished/Failed）时，服务额外推送 `final`，包含完整详情字段。
4) 轮询接口仍保留作为降级路径，SSE 主要用于低延迟实时更新。
