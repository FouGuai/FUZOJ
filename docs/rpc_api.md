# RPC 接口文档（服务内部调用）

本文档汇总服务内部 RPC（gRPC）接口，供服务间调用使用。接口定义来源于 `api/proto` 与对应实现代码，历史实现位于 `internal/**/rpc`，go-zero 服务迁移后可能位于 `services/**/internal/rpc`。

## Problem Service (gRPC)

Proto：`api/proto/problem/v1/problem.proto`

### ProblemService.GetLatest

- **用途**：查询题目的最新已发布元信息
- **请求**：`GetLatestRequest`
  - `problem_id`（int64，必填）
- **响应**：`GetLatestResponse`
  - `meta`（ProblemLatestMeta）
    - `problem_id`（int64）
    - `version`（int32）
    - `manifest_hash`（string）
    - `data_pack_key`（string）
    - `data_pack_hash`（string）
    - `updated_at`（int64，Unix 秒）

### 错误映射

实现位置：`internal/problem/rpc/server.go`

- `InvalidParams` → `InvalidArgument`
- `ProblemNotFound` / `NotFound` → `NotFound`
- `Unauthorized` → `Unauthenticated`
- `Forbidden` → `PermissionDenied`
- 其他 → `Internal`
