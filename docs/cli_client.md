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
  - `problem create|latest|delete|upload-prepare|upload-sign|upload-complete|upload-abort|publish`
  - `submit create|status|batch-status|source`
  - `judge status`

提示：`submit create` 可使用 `source_file=./main.cpp` 读取源码；`upload-complete` 支持 `parts_file`、`manifest_file`、`config_file` 读取 JSON 文件，避免在终端中直接粘贴长内容。

## 使用示例

```bash
# 启动 REPL
go run ./cmd/cli

# 登录并保存 token
user login username=demo password=secret

# 创建题目
problem create title="Two Sum" owner_id=1

# 提交代码（从文件读取）
submit create problem_id=1 user_id=2 language_id=cpp source_file=./main.cpp

# 上传数据包（prepare / sign / complete）
problem upload-prepare id=1 idempotency_key=idem-1 expected_size_bytes=1024 \
  expected_sha256=<sha> content_type=application/octet-stream created_by=1
```
