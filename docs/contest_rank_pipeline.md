# Contest Rank Pipeline

## 功能概览
该模块承接 Judge 最终状态事件，完成比赛资格校验、ICPC 计分、去重幂等与持久化，并异步投递已计分的 RankUpdateEvent 到 Kafka，由 Rank Service 写入排行榜缓存。链路采用事务出站（Outbox）+ 幂等保证最终一致，并支持多实例部署下的有序发送。

## 关键接口或数据结构
- Kafka 事件：
  - `judge.status.final`：Judge 最终状态事件（包含 contest_id/user_id/problem_id/created_at）。
  - `contest.rank.updates`：已计分事件，Rank Service 消费写入 Redis。
- 持久化表：
  - `contest_member_problem_state`：member + problem 维度状态（错误次数、首次 AC 时间、罚时等）。
  - `contest_member_summary_snapshot`：member 汇总快照（分数、罚时、AC 数、detail_json、版本号）。
  - `contest_rank_outbox`：事务出站事件表（pending/processing/sent + retry + lease）。
  - `contest_rank_outbox_lock`：contest 级发送租约锁，保证同一 contest 同时仅一个实例发送。
  - SQL 参考：`services/contest_service/schema_rank.sql`

## 使用示例或配置说明
1) Contest Service 订阅 `judge.status.final`，通过资格校验后更新 member 状态与汇总快照。  
2) 事务内写入 outbox，relay 先按 `contest_id` 抢租约，再按 `id asc` 串行投递 `contest.rank.updates`。  
3) Rank Service 消费后更新 ZSET 与 detail hash，完成榜单刷新。  

多实例下顺序策略：
- 同一 `contest_id`：单租约持有者发送，严格有序。  
- 不同 `contest_id`：可分散到不同实例并行处理。  
- 实例故障：租约超时后自动转移，`processing` 记录会回收为 `pending` 重试。  

配置示例：
- `judgeFinal.topic: judge.status.final`
- `rankUpdate.topic: contest.rank.updates`
- `judgeFinal.idempotencyTTL: 30m`
