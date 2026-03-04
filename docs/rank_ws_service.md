# Rank WS Service

## 1. 功能概览
Rank WS Service 提供排行榜的 WebSocket 推送能力，负责管理客户端订阅、发送快照与刷新通知。服务仅做读取与推送，不负责排行榜计算或写入，依赖 Redis 中的排行榜数据并通过 Pub/Sub 接收刷新信号。服务无状态，可水平扩展，适用于高并发连接场景。

## 2. 关键接口与数据结构
- WS：`GET /api/v1/contests/:id/leaderboard/ws`
  - Query：`page`、`page_size`、`mode`（`live`/`frozen`）
- Redis：
  - `contest:lb:{contestId}`（ZSET）
  - `contest:lb:detail:{contestId}:{memberId}`（HASH）
  - `contest:lb:meta:{contestId}`（version/updated_at）
  - Pub/Sub：`contest:lb:pubsub:{contestId}`

## 3. 使用说明
1) 客户端连接 WS 后，服务会发送一次 snapshot（整页数据）。
2) 收到 Redis Pub/Sub 刷新信号后，服务对订阅进行去抖并发送 refresh。
3) WS 服务可多实例部署，连接可无粘性调度，实例间无需共享订阅状态。
