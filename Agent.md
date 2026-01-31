# AI 编码与工程化规范指南 (Agent & Engineering Guidelines)

本指南旨在规范 AI 辅助编程时的代码质量、架构设计与工程习惯。请在生成代码、重构代码或设计架构时严格遵循以下原则。

## 1. 核心设计原则 (Core Design Principles)

### 1.1 可维护性优先 (Maintainability First)
*   **模块化 (Modularity)**: 严禁将所有逻辑堆砌在单个文件或单个函数中。每个包 (Package)、每个结构体 (Struct)、每个函数 (Function) 都应职责单一 (Single Responsibility Principle)。
*   **分层架构 (Layered Architecture)**: 严格遵守项目的分层规范（如 `Controller` -> `Service` -> `Repository`）。上层可调用下层，下层严禁反向调用上层，严禁跨层调用。
*   **认知负荷 (Cognitive Load)**: 单个函数的圈复杂度 (Cyclomatic Complexity) 不应过高。如果一个函数超过 50 行，大概率需要拆分。

### 1.2 可扩展性 (Extensibility)
*   **面向接口编程 (Interface Over Implementation)**: 
    *   在定义服务依赖时，优先使用 `Interface` 而非具体 `Struct`。
    *   *思考*: "如果未来要从 MySQL 切换到 MongoDB，或者要从本地缓存切换到 Redis，我的业务逻辑代码需要改动吗？" 如果需要大改，说明耦合太重。
*   **依赖注入 (Dependency Injection)**: 组件之间通过构造函数或 DI 容器注入依赖，禁止在业务逻辑内部直接实例化具体的外部服务类。
*   **配置解耦**: 所有的魔法值（Magic Numbers/Strings）、配置项（如超时时间、重试次数）必须抽取到配置文件或常量定义中，禁止硬编码。

### 1.3 简洁与无冗余 (DRY & KISS)
*   **DRY (Don't Repeat Yourself)**: 
    *   如果一段代码出现了两次，请考虑重构。
    *   如果一段代码出现了三次，**必须**重构为公共函数或组件。
*   **YAGNI (You Ain't Gonna Need It)**: 不要为了"未来可能用到"而编写复杂的死代码。只实现当前需求，但要为通过接口扩展留出余地，而不是提前写好实现。
*   **清理冗余**: 生成新代码时，自动检查并删除不再使用的 `import`、变量、死函数。

---

## 2. 具体的编码规范 (Coding Standards)

### 2.1 代码结构与风格
*   **Go 语言规范**: 严格遵循 `Effective Go` 和 `Uber Go Style Guide`。
    *   错误处理：优先返回 `error`，在非 `main` 包中严禁使用 `panic`（除非不可恢复的启动错误）。
    *   命名规范：变量名应具有描述性，`camelCase`；导出成员使用 `PascalCase`。
*   **文件组织**:
    *   `api/`: 定义接口协议 (Proto/OpenAPI)。
    *   `internal/`: 具体的业务逻辑，不对外暴露。
    *   `pkg/`: 可被外部通用的工具库。
    *   `configs/`: 配置文件模板。

### 2.2 错误处理与日志
*   **Wrap Errors**: 在层级调用中，使用 `fmt.Errorf("...: %w", err)` 包装错误上下文，而不是仅返回原始错误，以便追踪调用链。
*   **结构化日志**: 使用结构化日志库 (如 `zap` 或 `logrus`)。严禁使用 `fmt.Println` 打印生产环境日志。日志必须包含关键上下文 (如 `UserID`, `TraceID`, `RequestID`)。

### 2.3 测试 (Testing)
*   **单元测试 (Unit Tests)**: 核心业务逻辑（Service 层）必须包含单元测试。
*   **Table-Driven Tests**: 推荐使用表格驱动测试法，覆盖正常边界和异常边界。
*   **Mocking**: 对于外部依赖（数据库、网络请求），必须支持 Mock 以便隔离测试。

### 2.4 安全性 (Security)
*   **输入校验**: 所有外部输入（API 参数）默认不可信，必须进行严格校验。
*   **SQL 注入**: 严禁拼接 SQL 字符串，必须使用参数化查询或 ORM。
*   **敏感信息**: 密码、密钥严禁硬编码在代码库中。

---

## 3. 思考清单 (Pre-coding Checklist)
在生成任何代码之前，AI 必须进行以下自我检查：

1.  **这个改动属于哪个模块？** (是 User 域还是 Judge 域？)
2.  **这个函数是不是太长了？** (是否包含多层嵌套的一长串 `if-else`？)
3.  **这个变量名别人看得懂吗？** (避免使用 `a`, `b`, `flag` 这种无意义命名)
4.  **是否有硬编码的常量？**
5.  **如果需求变更，目前的写法好改吗？**
