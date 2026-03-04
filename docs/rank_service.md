# Rank Service

## 1. 功能概览
Rank Service 提供高性能排行榜存取能力，接收“已计分”的榜单更新事件并写入 Redis ZSET/Hash，支持 HTTP 分页查询，面向 10w 参赛规模与 1w QPS 查询场景进行优化。服务本身不负责赛制计算与判题规则，仅作为榜单存储与查询层，便于水平扩展与缓存优化。WebSocket 推送能力由独立的 Rank WS Service 承担。

## 2. 关键接口与数据结构
- HTTP：`GET /api/v1/contests/:id/leaderboard?page=&page_size=&mode=`
- Redis：
  - `contest:lb:{contestId}`（ZSET，member=member_id，score=sort_score）
  - `contest:lb:detail:{contestId}:{memberId}`（HASH，summary + per-problem detail）
  - `contest:lb:page:{contestId}:{mode}:{page}:{size}`（分页缓存）
  - `contest:lb:meta:{contestId}`（version/updated_at）

## 3. 使用说明
1) Rank 消费 Kafka 中的已计分事件，批量写入 Redis，并刷新榜单版本。
2) HTTP 查询优先走分页缓存，未命中则 ZREVRANGE + HGET 聚合返回。
3) WS 订阅与刷新由 Rank WS Service 负责，通过 Redis Pub/Sub 触发刷新。

> 说明：赛制逻辑（如首次 AC 生效）由 Contest Service 产出已计分事件实现，Rank 侧只做存取与推送。
