# FuzOJ 项目需求规格说明书 (PRD)

## 1. 项目概况 (Project Overview)
**FuzOJ** 是一个基于 **Golang** 开发的分布式、高并发在线判题系统 (Online Judge)。旨在为用户提供专业的算法训练环境、高质量的比赛竞技平台以及活跃的技术交流社区。核心设计目标是**高可用性**、**低延迟**和**极致的用户体验**。

---

## 2. 核心架构需求 (Core Architecture)

### 2.1 技术栈与架构 (Tech Stack)
*   **开发语言**: Golang (高性能、原生并发)。
*   **架构模式**: 微服务架构 (Microservices)。
*   **核心组件**:
    *   **Gateway**: 统一流量入口，鉴权与限流。
    *   **Service Mesh/RPC**: 高效服务间通信 (gRPC)。
    *   **Message Queue**: 削峰填谷 (Kafka/RabbitMQ)，用于判题任务调度。
    *   **Middleware**: Redis (缓存/榜单), MySQL (持久化), MinIO (对象存储).

### 2.2 分布式与高并发 (High Concurrency)
*   **水平扩展**: 判题机 (Judge Server) 需支持自动发现与水平扩容，应对万级并发提交。
*   **优先级队列 (Priority Queue)**: 
    *   判题任务进入消息队列时必须分级。
    *   **Level 0 (最高)**: 正在进行的比赛提交 (Contest Submission)。
    *   **Level 1**: 普通练习提交 (Practice Submission)。
    *   **Level 2**: 自测运行 (Custom Test)。
    *   **Level 3**: 后台重判任务 (Rejudge)。
    *   *目标*: 保证比赛期间系统负载满载时，选手依然能获得秒级反馈。

### 2.3 权限与角色 (Role Based Access Control)
*   **Super Admin**: 系统配置、全站管理。
*   **Admin**: 用户管理、公告发布。
*   **Problem Setter**: 题目创建、编辑、测试数据管理（仅对自己创建的题目可见）。
*   **Contest Manager**: 比赛创建、报名管理、处理申诉。
*   **User**: 普通做题、参赛。
*   **Guest**: 仅浏览公开题目和比赛。

---

## 3. 功能需求详情 (Functional Requirements)

### 3.1 判题系统 (The Judge System)

#### 3.1.1 核心判题
*   支持多种语言 (C, C++, Java, Go, Python, Rust, JavaScript 等)。
*   **安全沙箱**: 基于 Linux Cgroups 和 Namespaces (如 nsjail) 实现资源严格隔离，防止恶意代码攻击。
    *   **限制维度**: CPU 时间 (ms), 实际时间 (Wall time), 内存 (MB), 栈空间, 输出大小, 进程数。
    *   **系统调用过滤 (Seccomp)**: 白名单机制，禁止网络请求、文件读写（除标准流外）等危险操作。
*   **Special Judge 支持**: 
    *   原生支持 **Testlib.h** 协议。
    *   允许出题人上传自定义校验器 (Validator/Checker)，用于浮点数误差、多解等场景。
*   **交互题 (Interactive Problem) 支持**: 允许用户程序与系统提供的交互库（Interactor）进行实时输入输出交互。

#### 3.1.2 体验优化：自定义测试 (Custom Test)
*   **在线 IDE**: 题目详情页嵌入 Monaco Editor。
*   **自测运行**: 用户可输入自定义 Input 数据，点击“运行”，系统在一个轻量级容器中运行代码并返回 Output 和标准错误流 (Stderr)，**不计入判题统计**，不消耗正式比赛罚时。

#### 3.1.3 社交博弈：Hack 机制 (Hack Mechanism)
*   **Hack 周期**: 支持在比赛进行中（如 Codeforces 赛制）或赛后。
*   **Hack 流程**:
    1. 用户 A 通过某题。
    2. 用户 B 查看 A 的代码。
    3. 用户 B 提交一组能让 A 代码出错的数据 (Generator 或手动输入)。
    4. 系统验证 B 的数据是否符合题面约束 (Validator)。
    5. 系统用 B 的数据重判 A 的代码。
    6. 若 A 代码报错/超时，则 Hack 成功：A 被扣分，B 获得加分。

#### 3.1.4 判题结果定义 (Verdict Definitions)
*   明确定义系统判题状态，规范前后端交互与用户反馈：
    *   **Pending**: 排队等待中。
    *   **Running**: 正在判题中。
    *   **AC (Accepted)**: 通过，程序输出正确。
    *   **WA (Wrong Answer)**: 答案错误。
    *   **TLE (Time Limit Exceeded)**: 超出时间限制。
    *   **MLE (Memory Limit Exceeded)**: 超出内存限制。
    *   **RE (Runtime Error)**: 运行时错误（数组越界、除零等）。
    *   **CE (Compilation Error)**: 编译错误，详细展示编译器输出。
    *   **PE (Presentation Error)**: 格式错误（空格/换行问题，可视情况合并至 WA）。
    *   **SE (System Error)**: 系统内部错误（判题机故障），触发报警并自动重判。

### 3.2 比赛与榜单 (Contests & Leaderboard)

#### 3.2.1 赛制支持
*   **ICPC/ACM 赛制**: 实时封榜，罚时计算 (Time + Penalty)，样例全过才算 AC。
*   **IOI 赛制**: 子任务 (Subtasks) 得分，按测试点给分，多次提交取最高分。

#### 3.2.2 高性能实时榜单
*   利用 Redis `ZSET` 或自定义数据结构维护榜单。
*   支持**动态封榜** (Frozen Board)。
*   支持赛后**虚拟参赛 (Virtual Participation)**，重现比赛期间的榜单变化。

#### 3.2.3 观赛与直播 (Spectator Mode)
*   **实时观战**: 在比赛允许公开代码的情况下（或赛后），观众可以进入选手的 IDE 界面，实时观看其代码编写过程（类似 WebSocket 推送 Diff）。
*   **状态追踪**: 关注特定选手，当其提交代码时收到即时通知。
*   **比赛弹幕**: 观众可在榜单页或观战页发送实时弹幕，讨论赛况（支持开关屏蔽）。

### 3.3 题目管理与出题工厂 (Polygon Mode)

#### 3.3.1 题目仓库
*   完善的标签系统 (Tag System) 和难度分级。
*   全量题库检索。
*   **题目版本控制**: 记录题目描述、时限、内存限制的历史变更。

#### 3.3.2 测试数据管理
*   **手动上传**: 支持 Web 端拖拽上传 `.zip`压缩包形式的测试用例（包含 .in/.out 文件），支持大文件断点续传。
*   **Polygon 模式 (自动化)**: 解决“造数据难”的痛点。
    *   **生成器**: 出题人只需编写一个 Generator 代码 (C++/Python)。
    *   **脚本化**: 系统根据 Generator 和 Validator，在后台自动化运行并生成 50-100 组测试用例 (.in/.out)，并自动打包上传至对象存储 (MinIO)。
*   **数据预览与校验**: 在网页端直接预览前 1KB 的测试数据，并提供 md5 校验。

### 3.4 社区与教育 (Community & Education)

#### 3.4.1 AI 助教 (AI Copilot)
*   **主要功能**: 为练习模式提供智能提示。
*   **上下文感知**: AI 接收题目描述 + 用户当前错误代码 + 编译器报错信息。
*   **约束**: 严格的 Prompt Engineering，只解释思路、指出逻辑漏洞或语法陷阱，**禁止直接输出 AC 代码**。

#### 3.4.2 讨论与题解
*   Markdown 富文本编辑器。
*   支持 LaTeX 公式渲染。
*   题目维度的评论区与题解板块。

### 3.5 社交深度与游戏化 (Social Depth & Gamification)

#### 3.5.1 社交图谱与动态
*   **关注机制**: 支持用户互相关注。
*   **动态流 (Activity Feed)**: 首页展示关注对象的动态（通过题目、发布题解、比赛排名变化）。
*   **团队/公会 (Teams)**: 支持创建学校/组织维度的团队，拥有独立的私有题库、训练作业 (Homework) 和内部排名。

#### 3.5.2 实时互动与经济
*   **协同编程**: 支持多人实时编辑同一份代码（类似 Google Docs），方便 ICPC 队伍远程训练。
*   **FuzCoin 经济系统**: 通过 AC 题目、Hack 成功、贡献题解获取积分。

### 3.6 消息通知系统 (Notification System)

#### 3.6.1 全局通知中心
*   **多渠道触达**: 支持 Web 站内信 (WebSocket 实时推送)、邮件通知 (Email)。
*   **通知分类聚合**:
    *   **交互类**: 题解被点赞/收藏、评论回复、At (@) 提醒。
    *   **系统类**: 比赛开始提醒、公告更新、系统维护通知、封禁/警告通知。
    *   **状态类**: 提交代码判题完成（可配置）、Hack 成功/失败通知、工单回复。

---

## 4. 非功能性需求 (Non-Functional Requirements)

*   **稳定性**: 服务可用性目标 99.9%。
*   **可观测性**: 
    *   **Prometheus + Grafana**: 监控判题队列堆积量、判题机 CPU/内存、QPS。
    *   **Tracing**: 链路追踪，快速定位慢请求。
*   **安全性**: 
    *   代码执行严格限权 (UID/GID, Network, Filesystem)。
    *   DDoS 防护与 API 限流。
