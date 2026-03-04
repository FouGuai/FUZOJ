# Controller 接口文档（按 Service 划分）

本文档汇总 Controller/Handler 层接口能力，按 Service 归类，便于快速定位。历史代码位于 `internal/**/controller`，go-zero 服务位于 `services/**/internal/handler`，实际路由以对应服务的 `handler/routes.go` 为准。

## Problem Service

### ProblemController

- **Create**：创建题目基础信息，返回 `problem_id`
  - 请求体：`CreateProblemRequest`
    - `title`（string，必填）
    - `owner_id`（int64）
  - 响应体：`CreateProblemResponse`
    - `id`（int64）

- **GetLatest**：获取题目的最新已发布元信息
  - 路径参数：`id`（problem_id）
  - 响应体：`LatestMetaResponse`
    - `problem_id`（int64）
    - `version`（int32）
    - `manifest_hash`（string）
    - `data_pack_key`（string）
    - `data_pack_hash`（string）
    - `updated_at`（RFC3339 string）

- **GetStatement**：获取题目的最新已发布题面
  - 路径参数：`id`（problem_id）
  - 响应体：`StatementResponse`
    - `problem_id`（int64）
    - `version`（int32）
    - `statement_md`（string）
    - `updated_at`（RFC3339 string）

- **GetStatementVersion**：获取题目的指定版本题面
  - 路径参数：`id`（problem_id）、`version`
  - 响应体：`StatementResponse`
    - `problem_id`（int64）
    - `version`（int32）
    - `statement_md`（string）
    - `updated_at`（RFC3339 string）

- **UpdateStatement**：更新题目的指定版本题面（仅草稿版本可编辑）
  - 路径参数：`id`（problem_id）、`version`
  - 请求体：`UpdateStatementRequest`
    - `statement_md`（string，必填）
  - 响应：`SuccessWithMessage`

- **Delete**：删除题目
  - 路径参数：`id`（problem_id）
  - 响应：`SuccessWithMessage`

### ProblemUploadController

- **Prepare**：创建或复用上传会话，返回上传所需信息
  - Header：`Idempotency-Key`（必填）
  - 路径参数：`id`（problem_id）
  - 请求体：`PrepareUploadRequest`
    - `expected_size_bytes`（int64）
    - `expected_sha256`（string）
    - `content_type`（string）
    - `created_by`（int64）
    - `client_type`（string，必填，保留字段）
    - `upload_strategy`（string，必填，保留字段）
  - 响应体：`PrepareUploadResponse`
    - `upload_id`（int64）
    - `problem_id`（int64）
    - `version`（int32）
    - `bucket`（string）
    - `object_key`（string）
    - `multipart_upload_id`（string）
    - `part_size_bytes`（int64）
    - `expires_at`（RFC3339 string）

- **Sign**：为分片上传生成预签名 URL
  - 路由：`POST /api/v1/problems/:id/data-pack/uploads/:upload_id/sign`
  - 路径参数：`id`（problem_id）、`upload_id`
  - 请求体：`SignPartsRequest`
    - `part_numbers`（[]int，必填）
  - 响应体：`SignPartsResponse`
    - `urls`（map[string]string，key 为分片号字符串）
    - `expires_in_seconds`（int64）

- **Complete**：完成分片上传并落库元数据
  - 路由：`POST /api/v1/problems/:id/data-pack/uploads/:upload_id/complete`
  - 路径参数：`id`（problem_id）、`upload_id`
  - 请求体：`CompleteUploadRequest`
    - `parts`（[]CompletedPartInput）
    - `manifest_json`（JSON）
    - `config_json`（JSON）
    - `manifest_hash`（string）
    - `data_pack_hash`（string）
  - 响应体：`CompleteUploadResponse`
    - `problem_id`（int64）
    - `version`（int32）
    - `manifest_hash`（string）
    - `data_pack_key`（string）
    - `data_pack_hash`（string）

- **Abort**：中止上传会话
  - 路由：`POST /api/v1/problems/:id/data-pack/uploads/:upload_id/abort`
  - 路径参数：`id`（problem_id）、`upload_id`
  - 响应：`SuccessWithMessage`

- **Publish**：发布题目版本
  - 路由：`POST /api/v1/problems/:id/versions/:version/publish`
  - 路径参数：`id`（problem_id）、`version`
  - 响应：`SuccessWithMessage`

## User Auth Service

### AuthController

- **Register**：注册用户
  - 请求体：`RegisterRequest`
    - `username`（string，必填）
    - `password`（string，必填）
  - 响应体：`AuthResponse`
    - `access_token`
    - `refresh_token`
    - `access_expires_at`
    - `refresh_expires_at`
    - `user`（`UserInfo`：`id`、`username`、`role`）

- **Login**：登录
  - 请求体：`LoginRequest`
    - `username`（string，必填）
    - `password`（string，必填）
  - 响应体：`AuthResponse`

- **Refresh**：刷新访问令牌
  - 请求体：`RefreshRequest`
    - `refresh_token`（string，必填）
  - 响应体：`AuthResponse`

- **Logout**：注销（吊销 refresh token）
  - 请求体：`LogoutRequest`
    - `refresh_token`（string，必填）
  - 响应：`SuccessWithMessage`

## Submit Service

### SubmitController

- **Create**：提交代码并投递判题
  - Header：`Idempotency-Key`（可选）
  - 请求体：`SubmitRequest`
    - `problem_id`（int64，必填）
    - `user_id`（int64，必填）
    - `language_id`（string，必填）
    - `source_code`（string，必填）
    - `contest_id`（string，必填，可为空字符串）
    - `scene`（string，必填）
    - `extra_compile_flags`（[]string，必填，可为空数组）
  - 响应体：`SubmitResponse`
    - `submission_id`（string）
    - `status`（string）
    - `received_at`（unix ts）

- **BatchStatus**：批量获取提交状态
  - 请求体：`BatchStatusRequest`
    - `submission_ids`（[]string，必填）
  - 响应体：`BatchStatusResponse`
    - `items`（[]JudgeStatusResponse）
    - `missing`（[]string）
    - 默认返回摘要字段（不包含 `compile`/`tests`）
    - `items[].compile.Log`（string）：编译日志文本（最大 64KB，超出截断）
    - `items[].tests[].RuntimeLog`（string）：运行日志文本（最大 64KB，超出截断）
    - `items[].tests[].CheckerLog`（string）：Checker 日志文本（最大 64KB，超出截断）

- **GetSource**：获取提交源码
  - 路径参数：`id`（submission_id）
  - 响应体：`SourceResponse`
    - `submission_id`（string）
    - `problem_id`（int64）
    - `user_id`（int64）
    - `contest_id`（string）
    - `language_id`（string）
    - `source_code`（string）
    - `created_at`（RFC3339 string）

## Judge Service

### JudgeController

- **GetStatus**：获取单个提交状态（判题服务视角）
  - 路径参数：`id`（submission_id）
  - 响应体：`JudgeStatusResponse`
    - `compile.Log`（string）：编译日志文本（最大 64KB，超出截断）
    - `tests[].RuntimeLog`（string）：运行日志文本（最大 64KB，超出截断）
    - `tests[].CheckerLog`（string）：Checker 日志文本（最大 64KB，超出截断）

## Status Service

### StatusController

- **GetStatus**：获取单个提交状态（前端轮询入口）
  - 路径参数：`id`（submission_id）
  - Query：`include`（string，可选，取值 `details`/`log`）
  - 响应体：`JudgeStatusResponse`
    - 默认仅返回摘要字段（不包含 `compile`/`tests`）

## Rank Service

### RankController

- **Leaderboard**：获取排行榜分页数据
  - 路径参数：`id`（contest_id）
  - Query：`page`、`page_size`、`mode`（`live`/`frozen`）
  - 响应体：`LeaderboardResponse`

## Rank WS Service

### RankWSController

- **LeaderboardWS**：订阅排行榜刷新推送
  - 路径参数：`id`（contest_id）
  - Query：`page`、`page_size`、`mode`（`live`/`frozen`）
  - 协议：WebSocket（首次 snapshot + refresh 推送）
    - `include=details` 时返回 `compile`/`tests`
    - `compile.Log`（string）：编译日志文本（最大 64KB，超出截断）
    - `tests[].RuntimeLog`（string）：运行日志文本（最大 64KB，超出截断）
    - `tests[].CheckerLog`（string）：Checker 日志文本（最大 64KB，超出截断）

## Contest Service

### ContestController

- **Create**：创建比赛
  - 请求体：`CreateContestRequest`
    - `title`（string，必填）
    - `description`（string）
    - `visibility`（string）
    - `owner_id`（int64）
    - `org_id`（int64）
    - `start_at`（RFC3339 string）
    - `end_at`（RFC3339 string）
    - `rule`（ContestRulePayload）
  - 响应体：`CreateContestResponse`
    - `contest_id`（string）

- **Update**：更新比赛
  - 路径参数：`id`（contest_id）
  - 请求体：`UpdateContestRequest`

- **Publish**：发布比赛
  - 路径参数：`id`（contest_id）

- **Close**：关闭比赛
  - 路径参数：`id`（contest_id）

- **Get**：获取比赛详情
  - 路径参数：`id`（contest_id）
  - 响应体：`GetContestResponse`

- **List**：分页查询比赛列表
  - 请求参数：`page`、`page_size`、`status`、`owner_id`、`org_id`
  - 响应体：`ListContestsResponse`

- **Register**：报名参赛
  - 路径参数：`id`（contest_id）
  - 请求体：`RegisterContestRequest`

- **Approve**：审核报名
  - 路径参数：`id`（contest_id）
  - 请求体：`ApproveContestRequest`

- **Quit**：退赛
  - 路径参数：`id`（contest_id）
  - 请求体：`QuitContestRequest`

- **Participants**：参赛者列表
  - 路径参数：`id`（contest_id）
  - 请求参数：`page`、`page_size`
  - 响应体：`ListParticipantsResponse`

- **TeamCreate**：创建队伍
  - 路径参数：`id`（contest_id）
  - 请求体：`CreateTeamRequest`

- **TeamJoin**：加入队伍
  - 路径参数：`id`（contest_id）、`team_id`
  - 请求体：`JoinTeamRequest`

- **TeamLeave**：离开队伍
  - 路径参数：`id`（contest_id）、`team_id`
  - 请求体：`LeaveTeamRequest`

- **TeamList**：队伍列表
  - 路径参数：`id`（contest_id）
  - 请求参数：`page`、`page_size`

- **ProblemAdd**：绑定题目
  - 路径参数：`id`（contest_id）
  - 请求体：`AddContestProblemRequest`

- **ProblemUpdate**：更新题目编排
  - 路径参数：`id`（contest_id）、`problem_id`
  - 请求体：`UpdateContestProblemRequest`

- **ProblemRemove**：移除题目
  - 路径参数：`id`（contest_id）、`problem_id`

- **ProblemList**：题单列表
  - 路径参数：`id`（contest_id）
  - 响应体：`ListContestProblemsResponse`

- **Leaderboard**：实时榜单
  - 路径参数：`id`（contest_id）
  - 请求参数：`page`、`page_size`

- **LeaderboardFrozen**：封榜榜单
  - 路径参数：`id`（contest_id）
  - 请求参数：`page`、`page_size`

- **LeaderboardReplay**：回放榜单
  - 路径参数：`id`（contest_id）
  - 请求参数：`page`、`page_size`、`at`

- **MyResult**：个人最终结果
  - 路径参数：`id`（contest_id）
  - 响应体：`MyResultResponse`

- **MemberResult**：成员结果查询
  - 路径参数：`id`（contest_id）、`member_id`
  - 响应体：`MyResultResponse`

- **HackCreate**：发起 Hack
  - 路径参数：`id`（contest_id）
  - 请求体：`CreateHackRequest`

- **HackGet**：获取 Hack 详情
  - 路径参数：`id`（contest_id）、`hack_id`
  - 响应体：`GetHackResponse`

- **AnnouncementCreate**：发布公告
  - 路径参数：`id`（contest_id）
  - 请求体：`CreateAnnouncementRequest`

- **AnnouncementList**：公告列表
  - 路径参数：`id`（contest_id）
  - 请求参数：`page`、`page_size`

## Rank Service

### RankController

- **Leaderboard**：获取比赛排行榜分页
  - 路径参数：`id`（contest_id）
  - Query：`page` `page_size` `mode`(live|frozen)
  - 响应体：`LeaderboardResponse`
    - `items`（[]LeaderboardEntry）
    - `page`（PageInfo）
    - `version`（string）

- **LeaderboardWS**：订阅排行榜分页增量推送
  - 路径：`GET /api/v1/contests/:id/leaderboard/ws`
  - Query：`page` `page_size` `mode`
  - 推送：`snapshot` 首包 + `refresh` 更新包
