# CLI 调试客户端

## 功能概览

CLI 客户端提供类 telnet 的 REPL 交互方式，用于本地/测试环境快速构造请求并查看响应。输入格式为 `service action key=value`，缺失的必填字段会在 REPL 内提示补全，避免手写完整 JSON。默认通过网关地址 `http://127.0.0.1:8080` 调用各服务的 HTTP 接口，覆盖用户认证、题目管理与数据包上传、提交判题、判题状态查询等常用流程。CLI 会像前端一样保存 access token 到 `configs/cli_state.json`（可配置），并在需要鉴权的请求中自动携带 Authorization。输出默认包含状态码、耗时与格式化 JSON，便于快速定位接口错误或参数问题。

## 关键入口与配置

- 入口命令：`go run ./cmd/cli`
- 配置文件：`configs/cli.yaml`
  - `baseURL`：基础地址
  - `timeout`：HTTP 超时
  - `tokenStatePath`：token 状态文件（默认 `configs/cli_state.json`，已在 `.gitignore` 中忽略）
  - `prettyJSON`：是否格式化 JSON 输出
- 常用系统指令：
  - `help`、`exit`
  - `set base|timeout|token`
  - `show token|config`
- 主要命令：
  - `user register|login|refresh|logout`
  - `problem list|create|latest|statement|statement-version|statement-update|delete|upload-prepare|upload-sign|upload-complete|upload-abort|publish`
  - `submit create|batch-status|source`
  - `status status`
  - `judge status`
- `contest create|get|list|publish|close|register|participants|leaderboard|my-result`
  - `contest create` 必填：`title`、`start_at`、`end_at`（RFC3339 时间）
  - `contest list` 可选：`page`、`page_size`、`status`、`owner_id`、`org_id`
  - `contest get` 必填：`id`
- 判题状态相关返回：
  - `status status` 默认仅返回摘要字段（不含 compile/tests）
  - `status status include=details` 才返回 compile/tests
  - `status status` / `submit batch-status` / `judge status` 的 `data.compile.Log` 为编译日志文本（最大 64KB，超出截断）
  - `status status` / `submit batch-status` / `judge status` 的 `data.tests[].RuntimeLog` 为运行日志文本（最大 64KB，超出截断）
  - `status status` / `submit batch-status` / `judge status` 的 `data.tests[].CheckerLog` 为 Checker 日志文本（最大 64KB，超出截断）
- 数据包上传相关接口路径：
  - `POST /api/v1/problems/:id/data-pack/uploads/:upload_id/sign`
  - `POST /api/v1/problems/:id/data-pack/uploads/:upload_id/complete`
  - `POST /api/v1/problems/:id/data-pack/uploads/:upload_id/abort`
- 题目发布接口路径：
  - `POST /api/v1/problems/:id/versions/:version/publish`
- 公开题目列表接口路径：
  - `GET /api/v1/problems?limit=&cursor=`
- 题面接口路径：
  - `GET /api/v1/problems/:id/statement`
  - `GET /api/v1/problems/:id/versions/:version/statement`
  - `PUT /api/v1/problems/:id/versions/:version/statement`
- 排行榜 WS 接口由 Rank WS Service 提供，仅供前端/浏览器使用，CLI 暂不支持订阅
- 提交状态 SSE 接口由 Status SSE Service 提供，仅供前端/浏览器使用，CLI 暂不支持订阅

提示：REPL 支持方向键历史与行内编辑（仅当前会话生效）；`submit create` 可使用 `source_file=./main.cpp` 读取源码；`upload-complete` 支持 `parts_file`、`manifest_file`、`config_file` 读取 JSON 文件，避免在终端中直接粘贴长内容。

## 使用示例

```bash
# 启动 REPL
go run ./cmd/cli

# 登录并保存 token
user login username=demo password=secret

# 浏览公开题目列表
problem list limit=20

# 创建题目
problem create title="Two Sum" owner_id=1

# 提交代码（从文件读取）
submit create problem_id=1 user_id=2 language_id=cpp source_file=/home/foushen.zhan/fuzoj/tests/main.cpp

# 获取提交状态摘要
status status id=SUBMISSION_ID

# 获取提交状态详情（含 compile/tests）
status status id=SUBMISSION_ID include=details

# 上传数据包（prepare / sign / complete）
submit create problem_id=1 user_id=2 language_id=cpp source_file=./main.cpp
```
