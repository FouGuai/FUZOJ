# Contest 排行榜计分待办

## 目标
将赛制与计分逻辑集中在 Contest Service，实现“每题仅首次 AC 生效、后续不计罚时”的规则，并产出 Rank Service 可直接消费的已计分事件。

## 待办内容
1. **计分逻辑实现**
   - ICPC：首次 AC 生效；罚时策略支持 `icpc_standard`、`cf_style`。
   - IOI：多次提交取最高分，总分为各题最高分之和。

2. **已计分事件结构**
   - 产出字段：`contest_id`、`member_id`、`problem_id`、`sort_score`、`score_total`、`penalty_total`、`ac_count`、`detail_json`、`version`、`updated_at`。
   - 事件投递至 Kafka `contest.rank.updates`。

3. **幂等与重算**
   - 对同一提交/成员更新做幂等去重。
   - 支持比赛重判或规则变更后的榜单重算与回放。

4. **与 Rank Service 对接**
   - 确认 member_id 维度（目前按 user_id）。
   - 冻结榜/回放榜需要时生成对应事件。
