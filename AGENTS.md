# 仓库规范（Repository Guidelines）

## 项目结构与模块组织

- `cmd/`：各个服务的入口（例如 `cmd/gateway/main.go`、`cmd/user-service/main.go`）。
- `internal/`：核心业务逻辑；遵循分层调用链
  **Controller → Service → Repository**（禁止反向调用或跨层调用）。
- `pkg/`：可被其他模块复用的公共库；**统一错误码**存放在 `pkg/errors/`。
- `api/`：API 定义（OpenAPI / Proto 等规范）。
- `tests/`：自定义测试套件、测试辅助工具和示例。
- `docs/`、`examples/`、`deploy/`、`configs/`：文档、示例、部署资源和配置模板。

## 编码风格与命名规范

- 遵循 **Effective Go** 与 **Uber Go Style Guide**。
- 命名规范：
  - 局部变量使用 `camelCase`
  - 导出标识符使用 `PascalCase`

- 错误处理：
  - 返回 `error`，除 `main` 外禁止使用 `panic`
  - 使用 `fmt.Errorf("context: %w", err)` 包装错误

- 日志：
  - 使用结构化日志（如 zap / logrus）
  - 生产代码路径中禁止使用 `fmt.Println`

- **所有注释和错误信息必须使用英文**
- 业务错误必须使用 `pkg/errors/error.go` 与 `pkg/errors/code.go` 中定义的统一错误码

## 测试规范

- 测试代码统一放在 `tests/` 目录下，包含测试套件与辅助工具
- 优先使用**表驱动测试**
- 命名规范：
  - 文件以 `_test.go` 结尾
  - 测试函数命名为 `TestXxx`

- 外部依赖（数据库 / 网络等）必须 mock，保证单元测试隔离性
- CI 会在 push / PR 时自动运行测试并上传覆盖率产物

## 提交与 Pull Request 规范

- ai 不应该手动提交 git，应该由人类审核并且提交

## 安全与配置建议

- 所有外部输入必须校验
- 禁止使用字符串拼接构造 SQL
- 机密信息不得提交到仓库
- 使用环境变量或 `configs/` 中的配置模板管理敏感配置

## 语言使用约定（补充）

- **日常对话与讨论默认使用中文**
- **代码注释、错误信息、日志内容统一使用英文**

## 功能实现与代码复用

### 实现新功能前的必做检查

在开始实现任何新功能前，**必须** 在本文件的 **已实现的模块文档** 部分查询，检查是否已有可以复用的逻辑和组件：

例如以下检查：

- **缓存相关**：检查 `User Cache Repository` 或 `Common Cache Interface` 是否可用
- **数据访问**：检查 `User Repository` 或其他已有 Repository 的实现方式
- **事件处理**：检查是否已有消息队列消费者框架可复用
- **错误处理**：复用 `pkg/errors/` 中定义的统一错误码
- **通用工具**：查看 `pkg/` 下是否已有工具库可用

## 功能实现与文档规范

对于已实现的完整功能模块，必须编写相应文档并在本文件中注册。遵循以下规范：

### 文档编写要求

- 位置：统一放在 `docs/` 目录下，按功能模块分类
- 格式：`.md` 文件，使用中文编写
- 结构：分为三个层级
  - **第一层**：功能概览（用途、核心特性）
  - **第二层**：关键接口或数据结构（列出主要 API、类型定义）
  - **第三层**：使用示例或配置说明（可选，复杂模块必须包含）
- 长度：200-500 字为宜，简明扼要
- 示例见：`docs/user_cache_repository.md`

### 在 AGENTS.md 中的注册方式

- 在本文件末尾的 **已实现的模块文档** 部分添加条目
- 格式：模块名称 + 文档路径 + 一句话说明
- 示例：
  ```
  - **User Cache Repository**：`docs/user_cache_repository.md` — 缓存旁路模式辅助工具
  ```

## 已实现的模块文档

- **User Cache Repository**：`docs/user_cache_repository.md` — 缓存旁路模式实现，用于优化用户信息读取
- **Common Cache Interface**：`internal/common/cache/interface.go` — 统一的缓存操作接口定义
- **User Repository**：`docs/user_repository.md` — 用户、会话与缓存仓储的数据访问层实现
