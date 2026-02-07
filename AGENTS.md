# 仓库规范（Repository Guidelines）

## 项目结构与模块组织

* `cmd/`：各个服务的入口（例如 `cmd/gateway/main.go`、`cmd/user-service/main.go`）。
* `internal/`：核心业务逻辑；遵循分层调用链
  **Controller → Service → Repository**（禁止反向调用或跨层调用）。
* `pkg/`：可被其他模块复用的公共库；**统一错误码**存放在 `pkg/errors/`。
* `api/`：API 定义（OpenAPI / Proto 等规范）。
* `tests/`：自定义测试套件、测试辅助工具和示例。
* `docs/`、`examples/`、`deploy/`、`configs/`：文档、示例、部署资源和配置模板。

---

## 构建、测试与开发命令

* `go mod download` / `go mod verify`：下载并校验依赖。
* `go run ./cmd/gateway`：本地运行某个服务（将 `gateway` 替换为其他服务名）。
* `go build ./cmd/...`：构建所有服务的可执行文件。
* `go test ./tests/... -v`：运行 `tests/` 下的测试套件。
* `go test ./...`：运行所有包测试（用于 CI 覆盖率）。
* `go test ./tests/... -race -coverprofile=coverage.out`：开启竞态检测并生成覆盖率报告。
* `go tool cover -html=coverage.out -o coverage.html`：生成并查看本地覆盖率报告。
* `go vet ./...`：静态代码检查。
* `gofmt -w .`：格式化 Go 代码（CI 会强制校验）。

---

## 编码风格与命名规范

* 遵循 **Effective Go** 与 **Uber Go Style Guide**。
* 命名规范：

  * 局部变量使用 `camelCase`
  * 导出标识符使用 `PascalCase`
* 错误处理：

  * 返回 `error`，除 `main` 外禁止使用 `panic`
  * 使用 `fmt.Errorf("context: %w", err)` 包装错误
* 日志：

  * 使用结构化日志（如 zap / logrus）
  * 生产代码路径中禁止使用 `fmt.Println`
* **所有注释和错误信息必须使用英文**
* 业务错误必须使用 `pkg/errors/error.go` 与 `pkg/errors/code.go` 中定义的统一错误码

---

## 测试规范

* 测试代码统一放在 `tests/` 目录下，包含测试套件与辅助工具
* 优先使用**表驱动测试**
* 命名规范：

  * 文件以 `_test.go` 结尾
  * 测试函数命名为 `TestXxx`
* 外部依赖（数据库 / 网络等）必须 mock，保证单元测试隔离性
* CI 会在 push / PR 时自动运行测试并上传覆盖率产物

---

## 提交与 Pull Request 规范

* Git 提交记录使用**简短、小写**的摘要（无强制格式）
* 推荐使用祈使句 + 可选 scope，例如：

  * `user: add login handler`
* PR 需要包含：

  * 清晰的改动说明
  * 关联的 Issue（如有）
  * 测试命令与测试结果
* 涉及用户可见行为或逻辑变更时，附带截图或日志
* 合并前必须确保 CI 全部通过（测试、`go vet`、`gofmt`）

---

## 安全与配置建议

* 所有外部输入必须校验
* 禁止使用字符串拼接构造 SQL
* 机密信息不得提交到仓库
* 使用环境变量或 `configs/` 中的配置模板管理敏感配置

---

## 语言使用约定（补充）

* **日常对话与讨论默认使用中文**
* **代码注释、错误信息、日志内容统一使用英文**
