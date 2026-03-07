# Contest Service 设计文档

## 1. 功能概览
Contest Service 用于统一管理比赛全生命周期与赛制逻辑，覆盖比赛信息管理、报名与参赛、题目编排、提交入口、排行榜、封榜、虚拟参赛、Hack 机制与观赛模式。服务目标是提供高并发、低延迟、可扩展的比赛体验，并与 Submit/Judge/Gateway/Problem/User 等服务协作。

## 2. 核心功能清单
### 2.1 比赛生命周期
- 创建/编辑/发布/关闭比赛
- 比赛模板（赛制、时长、罚时、封榜策略、题数/分组）
- 比赛阶段控制：预热期、进行中、封榜期、结束后、公示期
- 支持公开/私有/邀请码/组织内比赛

### 2.2 参赛与权限
- 报名与审核（自动/手动）
- 参赛资格校验（时间窗口、权限、黑名单、组织成员）
- 角色：Contest Manager/Problem Setter/Participant/Spectator
- 团队赛支持：队伍创建、成员变更、队伍报名、队伍排名

### 2.3 题目编排与版本
- 比赛题单维护（题目顺序、分组、题面显示策略）
- 题目版本绑定：冻结题面/时限/数据包版本
- 题目可见性控制：比赛期间可见/赛后公开

### 2.4 赛制支持
- **ICPC/ACM**：首次 AC 计分；罚时 = 通过时间 + 错误提交罚时
- **IOI**：子任务得分；多次提交取最高分
- **Codeforces 风格**：动态评分、Hack 机制、补题/重判
- 可扩展赛制：通过策略模式扩展计分与榜单逻辑

### 2.5 排行榜与封榜
- 实时榜单（Redis ZSET/Hash + 本地缓存）
- 动态封榜：封榜后仅显示封榜前排名
- 多维榜单：个人榜、队伍榜、组织榜、滚榜
- 回放榜单：虚拟参赛与比赛回放

### 2.6 提交入口与判题协作
- 比赛提交入口与状态跟踪
- Submit Service 作为统一提交入口，Contest 仅做规则与资格校验
- 异步判题结果回传（消息队列），Contest 订阅并更新榜单
- 与 Judge Service 联动：赛制特定判题规则与加权
- 重判与数据包更新后的排行榜重算

### 2.7 Hack 与观赛
- Hack 申请、验证、重判、计分调整
- Hack 数据生成与约束校验
- 观赛模式：代码可见性控制与时间轴回放

### 2.8 通知与公告
- 比赛公告/滚动消息
- 开赛/封榜/结束提醒
- 参赛资格变更与审核通知

## 3. 关键赛制逻辑
### 3.1 ICPC/ACM
- 仅首次 AC 计分
- 罚时规则：`AC 提交时间 + 错误提交次数 * PenaltyMinutes`
- 每题状态：未提交/尝试中/已通过

### 3.2 IOI
- 记录多次提交的最高分
- 总分为各题最高分之和
- 可配置子任务权重、测试点计分方式

### 3.3 Codeforces/Hack
- 动态分值与罚时
- Hack 成功：被 Hack 选手扣分；发起者得分
- Hack 失败：发起者罚时或扣分（可配置）

## 4. 排行榜计算与缓存
- Cache-Aside：优先读取 Redis 缓存，未命中才计算
- 空值缓存：无排名也缓存占位，TTL 5 分钟
- 热点榜单：本地缓存 + Redis 双层缓存
- 排名重算：支持全量与增量重算
- 高并发优化：批量读取（MGET/PIPELINE），减少 RTT

## 5. 高性能排行榜设计（重点）
### 5.1 存储结构
- `contest:lb:{contestId}`：Redis ZSET，member 为 userId 或 teamId，score 为主排序分
- `contest:lb:detail:{contestId}:{memberId}`：Redis HASH，存储题目状态、罚时、最后提交时间等
- `contest:lb:meta:{contestId}`：Redis HASH，存储榜单版本、最后刷新时间、封榜状态
- `contest:lb:frozen:{contestId}`：封榜快照 ZSET
- 可选分页缓存：`contest:lb:cache:{contestId}:{page}`，短 TTL

### 5.2 写路径（Judge 结果）
- 判题结果走消息队列（Kafka），Contest Service 消费后更新榜单
- 使用 pipeline 批量写入 Redis，降低 RTT
- 对同一 member 的重复判题做幂等去重
- 写入路径支持 50-100ms 内存合并，降低热点抖动

### 5.3 读路径
- 本地缓存 TopN 与热门分页（TTL 2-5s）
- Redis ZSET/HASH 为权威实时数据源
- TopN：`ZREVRANGE` + `HMGET` 批量取详情
- 分页：`ZREVRANGE` + `HMGET`
- 单用户：`ZREVRANK` + `HGETALL`

### 5.4 计分编码建议
- ICPC：score 编码为 `ACCount * 1e12 - PenaltyTime`，保证主排序与次排序
- IOI：score 为总分，题目分细节放 HASH

### 5.5 封榜与切换
- 封榜开始时生成 `contest:lb:frozen:{contestId}` 快照
- 封榜期间仅更新真实榜单，对外返回 frozen
- 解封时切回真实榜，必要时用版本号原子切换

### 5.6 重算与补偿
- 支持全量重算与增量重算
- 重算期间可读旧榜，重算完成后切换榜单版本
- 以提交记录回放生成 ZSET，保证一致性

## 6. 关键数据模型（建议）
- Contest：基础信息、赛制、时间窗口、状态
- ContestProblem：题目绑定、排序、版本、分值
- ContestParticipant：参赛记录、角色、资格
- ContestTeam：队伍信息与成员
- ContestSubmission：提交与判题结果摘要
- ContestScore：榜单快照与增量记录
- ContestFreeze：封榜策略与执行状态
- ContestAnnouncement：公告内容与时间

## 7. 关键接口（建议）
- 比赛管理：创建/编辑/发布/关闭/复制
- 参赛管理：报名/审核/退赛/队伍管理
- 题目管理：题单维护/题目版本锁定
- 榜单查询：实时榜/封榜榜/回放榜
- Hack：发起/验证/结果查询
- 通知与公告：发布/查询

## 8. REST API 设计（Contest Service 对外）
所有接口统一走 Gateway，使用 JWT 鉴权与限流。
### 8.1 比赛管理
- `POST /api/v1/contests`：创建比赛
- `PUT /api/v1/contests/:id`：编辑比赛
- `POST /api/v1/contests/:id/publish`：发布比赛
- `POST /api/v1/contests/:id/close`：关闭比赛
- `GET /api/v1/contests/:id`：获取比赛详情
- `GET /api/v1/contests`：分页查询比赛列表

#### 8.1.1 CRUD 行为说明
- 创建比赛必填：`title`、`start_at`、`end_at`（RFC3339），且 `start_at` 必须早于 `end_at`
- 更新比赛为 Patch 语义：仅提交字段会覆盖，未提交字段保持不变
- 规则字段以 `rule` 聚合保存为 JSON；未传 `rule_type` 默认按 `icpc` 处理
- 列表分页 `page_size` 超过配置上限时会被截断

### 8.2 参赛与权限
- `POST /api/v1/contests/:id/register`：报名参赛
- `POST /api/v1/contests/:id/approve`：审核报名
- `POST /api/v1/contests/:id/quit`：退赛
- `GET /api/v1/contests/:id/participants`：参赛列表

### 8.3 团队赛
- `POST /api/v1/contests/:id/teams`：创建队伍
- `POST /api/v1/contests/:id/teams/:team_id/join`：加入队伍
- `POST /api/v1/contests/:id/teams/:team_id/leave`：离开队伍
- `GET /api/v1/contests/:id/teams`：队伍列表

### 8.4 题目编排
- `POST /api/v1/contests/:id/problems`：绑定题目
- `PUT /api/v1/contests/:id/problems/:problem_id`：更新排序/分值/可见性
- `DELETE /api/v1/contests/:id/problems/:problem_id`：解绑题目
- `GET /api/v1/contests/:id/problems`：获取题单

### 8.5 榜单
- `GET /api/v1/contests/:id/leaderboard`：实时榜
- `GET /api/v1/contests/:id/leaderboard/frozen`：封榜榜单
- `GET /api/v1/contests/:id/leaderboard/replay`：回放榜单（时间点参数）
- `GET /api/v1/contests/:id/my_result`：个人最终结果
- `GET /api/v1/contests/:id/results/:member_id`：管理员/队长查询结果

### 8.6 Hack 与公告
- `POST /api/v1/contests/:id/hacks`：发起 Hack
- `GET /api/v1/contests/:id/hacks/:hack_id`：Hack 详情
- `POST /api/v1/contests/:id/announcements`：发布公告
- `GET /api/v1/contests/:id/announcements`：公告列表

## 9. RPC 设计（服务间调用，go-zero zrpc）
### 9.1 Contest Service 对外 RPC
- `CheckParticipation(ContestId, UserId, ProblemId)`：校验参赛资格与题目可提交
- `GetContestMeta(ContestId)`：获取比赛元信息（时间、赛制、封榜策略）
- `GetContestRule(ContestId)`：获取赛制规则与罚时逻辑
- `GetContestProblems(ContestId)`：获取题单与可见性

### 9.2 Contest Service 消费的 RPC/HTTP
- Submit Service
  - `CreateSubmission(...)`：提交代码（仅在必要时使用）
  - `BatchStatus(SubmissionIds[])`：批量拉取状态
- Problem Service
  - `GetLatestProblemMeta(ProblemId)`：题目版本
  - `BatchGetProblemMeta(ProblemIds[])`
- User Service
  - `BatchGetUserProfiles(UserIds[])`
  - `CheckUserRole(UserId, Role)`

### 9.3 Judge 结果回传（MQ + RPC 补偿）
- 消息队列事件：`judge.finished`
- RPC 补偿接口：`GetSubmissionBrief(SubmissionId)`

## 10. 与现有服务交互与接口约定
### 8.1 与 Submit Service
- 提交入口统一由 Submit Service 提供
- Submit 在创建提交时，若 `contest_id` 非空，调用 Contest 做资格与题目校验
- 结果查询使用 Submit 的批量接口，减少单次调用

### 8.2 与 Judge Service
- 判题结果通过消息队列异步回传
- Contest 订阅 `judge.finished` 事件，仅处理 `contest_id` 非空的结果
- 重判通过 Judge Service 的批量重判接口触发

### 8.3 与 Problem/User/Gateway
- Problem Service：获取题目版本、题面元信息
- User Service：用户、组织与队伍关系校验
- Gateway：统一鉴权与限流

### 8.4 事件约定（Kafka）
- `contest.submission.created`
- `judge.finished`
- `contest.hack.finished`
- `contest.leaderboard.rebuild`

## 11. 观测与安全
- 关键日志：创建/报名/提交/榜单重算/封榜事件
- 指标：榜单刷新耗时、缓存命中率、提交吞吐
- 权限校验：强制鉴权、接口级限流
- 输入校验：题目与比赛 ID、时间窗口、参赛资格

## 12. 后续实现建议
- 优先落地：比赛创建 + 报名 + ICPC 榜单
- 第二阶段：封榜 + IOI 计分
- 第三阶段：Hack + 观赛 + 回放

## 13. 缺少项的详细设计
### 11.1 赛制规则配置化
为不同赛制提供统一的配置结构，落库并支持版本化与灰度发布。
- ContestRule（核心字段）
  - `rule_type`：icpc/ioi/cf/custom
  - `penalty_minutes`：ICPC 罚时（默认 20）
  - `penalty_formula`：罚时公式（支持表达式或策略名称）
  - `penalty_cap_minutes`：罚时上限（可选）
  - `freeze_minutes_before_end`：封榜时长
  - `allow_hack`：是否允许 Hack
  - `hack_reward` / `hack_penalty`：Hack 成功/失败加减分或罚时
  - `max_submissions_per_problem`：单题提交上限（可选）
  - `score_mode`：IOI 子任务计分模式（sum/max/weighted）
  - `publish_solutions_after_end`：赛后是否公开题解/代码
  - `virtual_participation_enabled`：是否允许虚拟参赛
- 规则变更流程
  - 仅在比赛未开始或由管理员强制变更
  - 变更后生成新版本号，写入 `contest_rule_version`

### 11.2 比赛状态机
状态机确保比赛生命周期操作幂等，避免并发/重复请求导致非法状态。
- 状态定义：`draft` → `published` → `running` → `frozen` → `ended` → `archived`
- 允许转换
  - `draft` → `published`
  - `published` → `running`
  - `running` → `frozen`（满足封榜条件）
  - `running`/`frozen` → `ended`
  - `ended` → `archived`
- 幂等控制
  - 每次转换携带 `Idempotency-Key`
  - 存储 `transition_log`，若重复 key 则返回上次结果

### 11.3 一致性与幂等设计
用于判题回调、榜单更新、Hack 结果更新。
- 幂等键建议
  - `judge_event_id`：由 Judge 生成（submission_id + attempt + verdict）
  - `hack_event_id`：hack_id + target_submission_id
- 数据写入顺序
  1. 幂等检查（Redis SETNX 或 DB 唯一索引）
  2. 更新成员题目状态
  3. 计算新分值并写 ZSET
- 并发处理
  - 以 `contest_id + member_id` 为粒度的细粒度锁（Redis 分布式锁，短 TTL）

### 11.4 失败补偿与消息队列策略
保障判题结果与榜单更新可靠到达。
- Kafka 消费失败策略
  - retry 3-5 次（指数退避）
  - 超过阈值写入 DLQ（Dead Letter Queue）
  - 定期任务扫描 DLQ 重放
- 幂等保证
  - DLQ 重放与重复事件不会导致分值重复累计

### 11.5 访问控制与可见性策略
细化比赛期间题目与代码的可见性，避免越权。
- 题面可见性
  - `visible_before_start`：预热期是否可见
  - `visible_during_contest`：比赛中可见
  - `visible_after_end`：赛后公开
- 代码可见性
  - `code_visible_during_contest`：比赛中是否可见
  - `code_visible_after_end`：赛后是否可见
- 角色策略
  - 仅 Contest Manager/Problem Setter 可见全量代码
  - Spectator 仅可见允许公开的题面与榜单

### 11.6 榜单落库与结算策略
榜单需要长期持久化，作为长期可查询的官方排名与审计依据。
- 快照策略
  - 每 5-10 分钟生成快照（或关键事件触发）
  - 结赛时生成最终榜单快照
- 持久化与长期保留
  - 结赛后生成 `contest_final_rank`（长期存储，不随缓存失效）
  - 历史榜单查询默认读取持久化表，保证稳定与可追溯
- 落库内容
  - `contest_score_snapshot`：member_id、score、rank、detail_json
  - `contest_final_rank`：最终排名、奖项标识、领奖信息
- 使用场景
  - 赛后复查、申诉处理、导出

### 11.8 比赛最终结果与个人明细存储
确保每个用户可长期查询自己的 AC 题目与罚时明细。
- 最终结果表（建议）
  - `contest_final_result`：member_id、rank、score、total_penalty、ac_count、detail_json、awards_json
  - `detail_json` 建议包含：
    - 每题状态（AC/WA/未尝试）
    - 首次 AC 时间
    - 错误提交次数
    - 题目罚时与累计罚时
- 查询接口
  - `GET /contests/:id/my_result`：返回个人最终结果与题目明细
  - `GET /contests/:id/results/:member_id`：管理员或队长查询

### 11.9 罚时逻辑可配置
罚时规则必须支持自定义并可版本化。
- 方式一：配置表达式（安全解析）
  - 示例：`penalty = ac_time + wrong_count * penalty_minutes`
  - 支持可选变量：`ac_time`、`wrong_count`、`hack_penalty`、`resubmit_penalty`
- 方式二：策略枚举
  - `icpc_standard`、`icpc_no_penalty_before_ac`、`cf_style` 等
- 版本化与回放
  - 规则变更生成新版本，写入 `contest_rule_version`
  - 回放与争议处理时使用当时规则版本

### 11.7 回放与虚拟参赛
提供赛况回放与虚拟参赛时的相对时间计算。
- 时间轴
  - `contest_start_at` 作为基准
  - 提交事件按相对时间回放
- 虚拟参赛
  - `virtual_start_at` 记录用户开始时间
  - 榜单查询时基于 `virtual_start_at` 过滤提交
- 回放榜单
  - 根据快照或回放事件生成指定时间点榜单
