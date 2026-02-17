# Controller 接口文档（按 Service 划分）

本文档汇总 `internal/**/controller` 下的接口能力，按 Service 归类，便于快速定位。接口的实际路由路径未在 controller 文件中定义，请以路由注册处为准。

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
    - `client_type`（string，保留字段）
    - `upload_strategy`（string，保留字段）
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
    - `contest_id`（string，可选）
    - `scene`（string，可选）
    - `extra_compile_flags`（[]string，可选）
  - 响应体：`SubmitResponse`
    - `submission_id`（string）
    - `status`（string）
    - `received_at`（unix ts）

- **GetStatus**：获取单个提交状态
  - 路径参数：`id`（submission_id）
  - 响应体：`JudgeStatusResponse`

- **BatchStatus**：批量获取提交状态
  - 请求体：`BatchStatusRequest`
    - `submission_ids`（[]string，必填）
  - 响应体：`BatchStatusResponse`
    - `items`（[]JudgeStatusResponse）
    - `missing`（[]string）

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
