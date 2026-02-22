# 仓库规范（Repository Guidelines）

## 项目结构与模块组织

- `services/`：go-zero 服务入口与代码（例如 `services/judge_service/`、`services/user_service/`）。
  - 每个服务是独立的 go-zero 项目结构，目录结构：`internal/handler` → `internal/logic` → `internal/repository` → `internal/model`，依赖由 `internal/svc` 注入。
- `internal/`：历史核心业务逻辑；严格遵循分层调用链
  **Controller → Logic → Repository → Model**（禁止反向调用或跨层调用）。
- `pkg/`：可被其他模块复用的公共库；**统一错误码**存放在 `pkg/errors/`。
- `api/`：API 定义（OpenAPI / Proto 等规范）。
- `tests/`：自定义测试套件、测试辅助工具和示例。
- `docs/`、`examples/`、`deploy/`、`configs/`：文档、示例、部署资源和配置模板。

## 编码风格与命名规范

- 遵循 **Effective Go** 与 **Uber Go Style Guide**。
- 新增代码必须遵循 **go-zero** 的项目结构与编码规范。
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
- 在关键地方写上清楚的注释
- **除非我明确指定，否则运行时配置信息不能从环境变量中获取，优先从配置文件中获取**

## 测试规范

- 每完成一个模块都需要写一套完整测试
- 测试代码统一放在 `tests/` 目录下，包含测试套件与辅助工具
- 优先使用**表驱动测试**
- 命名规范：
  - 文件以 `_test.go` 结尾
  - 测试函数命名为 `TestXxx`

- 外部依赖（数据库 / 网络等）必须 mock，保证单元测试隔离性
- CI 会在 push / PR 时自动运行测试并上传覆盖率产物
- 如果完成整个模块请写一个整个模块的端到端测试

## 提交与 Pull Request 规范

- AI 不应该手动提交 git，应该由人类审核并提交

## 性能要求

该项目并发访问高，设计时应当极其注意性能

### 缓存优先原则

- **有缓存就要用缓存**：除非有明确说明，否则任何可以缓存的数据都应该使用缓存机制
  - 用户信息、配置数据 → Redis 缓存（TTL: 30min）
  - 黑名单数据（被封禁用户） → 本地内存缓存 + Redis 备份
  - 热点数据 → 多层缓存（本地内存 → Redis → 数据库）
  - 频繁查询的数据 → 一定要缓存
  - 要考虑缓存雪崩、缓存穿透、缓存击穿的影响

- **缓存模式遵循规范**：
  - 读操作：使用 go-zero `cache.Cache` 的 `Take` 实现 Cache-Aside 模式
  - 空值缓存：防止缓存穿透，用占位值 + 短 TTL（如 5 分钟）缓存空结果
  - 写操作：使用 Write-Through（写入后立即删除缓存）或 Write-Behind（异步更新）

- **缓存框架和工具**：
  - 新代码优先使用 go-zero `cache.Cache` / `cache.CacheConf` 与 model 层 `sqlc.CachedConn`
  - 历史模块可参考 **已实现的模块文档** 中的 `Common Cache Interface`
  - 复用现有的缓存实现，不要重复造轮子


### 高并发优化

- **连接池**：数据库、Redis、MQ 都要使用连接池，单个连接池共享（不是一个 Repository 一个池）
- **批量操作**：大量数据查询时使用批量接口（`MGET`、`IN` 查询），减少网络往返
- **异步处理**：非关键路径的操作（日志、审计、通知）用消息队列异步处理
- **超时控制**：所有外部调用（数据库、缓存、MQ）都要设置 context timeout，防止慢查询堆积

### 监控与可观测性

- 添加性能相关的日志和 metrics（查询耗时、缓存命中率、慢查询）
- 定期查看是否有性能瓶颈（未缓存的热点数据、N+1 查询等）

## 安全与配置建议

- 所有外部输入必须校验
- 禁止使用字符串拼接构造 SQL
- 机密信息不得提交到仓库
- 敏感配置使用环境变量或 `configs/` 中的配置模板管理（但除非明确指定，不从环境变量读取运行时配置）

## 数据库设计
数据库设计不能使用外键；对于外键的语义，应当在业务逻辑中显式实现

## 接口设计
接口应该简洁、幂等，只实现一种语义
- 对于服务内部的接口请使用 gRPC 调用
- 前端调用使用 HTTPS

## 语言使用约定

- **日常对话与讨论默认使用中文**
- **代码注释、错误信息、日志内容统一使用英文**

## 功能实现与代码复用

### 实现新功能前的必做检查

在开始实现任何新功能前，**必须** 在本文件的 **已实现的模块文档** 部分查询，检查是否已有可以复用的逻辑和组件：

例如以下检查：

- **缓存相关**：优先检查 go-zero model 缓存与 `cache.Cache`，历史模块再参考 `Common Cache Interface`
- **数据访问**：检查 `User Auth Service` 或其他已有 Repository 的实现方式
- **事件处理**：检查是否已有消息队列消费者框架可复用
- **错误处理**：复用 `pkg/errors/` 中定义的统一错误码
- **通用工具**：查看 `pkg/` 下是否已有工具库可用

###  日志要求
**在关键代码处，请打印详细的日志，以便调试使用**
- 在所有的异常分支都要加日志
- 在任务的开启处要加日志

## 功能实现与文档规范

对于已实现的完整功能模块，必须编写相应文档并在本文件中注册。遵循以下规范：

### 文档编写要求

- 位置：统一放在 `docs/` 目录下，按功能模块分类
- 格式：`.md` 文件，使用中文编写
- 复用性：有些功能相似的模块可以放在同一个文件中
- 结构：分为三个层级
  - **第一层**：功能概览（用途、核心特性）
  - **第二层**：关键接口或数据结构（列出主要 API、类型定义）
  - **第三层**：使用示例或配置说明（可选，复杂模块必须包含）
- 长度：200-500 字为宜，简明扼要
- 示例见：`docs/user_auth_service.md`


### 接口文档
- **Controller API Index**：`docs/controller_api.md` — Controller 层接口汇总（按 Service 划分）
- 接口请放在 `docs/controller_api.md`
- **新增/变更 Controller 接口时必须同步更新 CLI 调试客户端与 `docs/cli_client.md`**
- **新增服务时必须同步更新启动脚本（如 `scripts/start_services.sh`）**

### 在 AGENTS.md 中的注册方式

- 在本文件末尾的 **已实现的模块文档** 部分添加条目
- 格式：模块名称 + 文档路径 + 一句话说明
- 示例：
  ```
  - **User Auth Service**：`docs/user_auth_service.md` — 用户认证、仓储与 Token/黑名单管理的模块说明
  ```

## 已实现的模块文档

- **Common Cache Interface**：`internal/common/cache/interface.go` — 统一的缓存操作接口定义（历史模块）
- **User Auth Service**：`docs/user_auth_service.md` — 用户认证、仓储与 Token/黑名单管理的模块说明
- **Sandbox Engine**：`docs/sandbox_engine.md` — Linux 原生沙箱引擎与初始化流程说明
- **Sandbox Runner**：`docs/sandbox_runner.md` — 判题编排层，负责生成 RunSpec 并执行编译、运行与 SPJ
- **Judge Worker**：`docs/judge_worker.md` — 沙箱执行调度单元，负责编译、运行与 SPJ 结果汇总
- **Problem Module**：`docs/problem_module.md` — 题目元信息管理与数据包上传发布流程说明
- **Judge Service**：`docs/judge_service.md` — Kafka 判题消费、数据包缓存与状态机查询服务
- **Submit Service**：`docs/submit_service.md` — 面向前端的判题入口与提交分发服务
- **Gateway**：`docs/gateway.md` — 统一流量入口的鉴权、限流与反向代理网关
- **CLI Client**：`docs/cli_client.md` — 面向联调用的命令行客户端，覆盖主要 HTTP 接口
- **ELK Logging**：`docs/logging_elk.md` — 结构化日志、采集与检索规范
