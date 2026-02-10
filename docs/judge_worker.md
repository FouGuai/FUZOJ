# Judge Worker

## 功能概览
Judge Worker 是沙箱执行的调度单位，负责接收上层准备好的判题请求并驱动编译、运行与 SPJ 校验流程。Worker 不拉取题目数据，所有输入文件路径由上层准备好并传入，从而保持沙箱层无状态、轻依赖。Worker 内部会创建与清理提交级工作目录，确保执行环境隔离；对于编译语言，仅执行一次编译并在每个测试点目录复用产物，避免重复编译带来的性能开销。执行策略默认“首个非 AC 即停”，并支持子任务分组评分（组内任一非 AC 则该组 0 分）。Worker 会输出测试点明细与汇总统计，便于上层持久化与回调，也能用于后续的性能观测与结果追踪。
此外，Worker 只聚焦执行与判定，不负责题目数据下载、缓存管理与调度策略；这些能力由上层服务或 Dispatcher 负责。这样可以让 Worker 轻量化、易水平扩展，配合 Worker Pool 实现并行执行与负载隔离。出现系统性错误时，Worker 以统一错误码返回，方便上层进行重试与告警。

## 关键接口与数据结构
- `Worker.Execute(ctx, req)`：执行一次完整判题流程，返回 `JudgeResult`。
- `JudgeRequest`：判题请求载体，包含 `SubmissionID/LanguageID/WorkRoot/SourcePath/Tests/Subtasks` 等字段。
- `TestcaseSpec`：单个测试点描述，包含输入/答案路径、IO 配置、资源限制、分值与可选 SPJ。
- `SubtaskSpec`：子任务分组规则，支持 `min` 评分策略。
Worker 通过配置仓库加载 `LanguageSpec` 与 `TaskProfile`，并将资源限制按“测试点覆盖 > Profile 默认值 > 语言倍率”的顺序合并，最终交由 Runner 构建 `RunSpec` 并调用 Engine 执行。

## 使用说明（示例）
1) 上层准备好源码与测试数据文件路径，并构造 `JudgeRequest`，每个测试点可配置 IO 模式与资源限制。
2) Worker 加载语言与 Profile 配置，执行编译与逐点运行；若遇到非 AC，则按策略提前终止后续测试点。
3) 返回 `JudgeResult`，包含编译结果、测试点明细与汇总统计；上层可据此更新状态机、落库与通知。
