# Sandbox Runner

## 功能概览

Sandbox Runner 是判题流程中的编排层，负责将语言配置、任务 Profile 与测试数据转化为 `RunSpec`，并调用 Sandbox Engine 执行编译、运行与 SPJ（交互题暂未实现但已预留扩展点）。当前实现以 C++ 为优先支持语言，通过“基础请求结构 + 语言扩展结构”的方式支撑后续多语言横向扩展。Runner 采用统一的资源限制与判定规则：优先使用请求覆盖值，其次使用 Profile 默认限制，同时应用语言倍率（Time/Memory multiplier）。标准输入输出默认使用 stdio 模式，可扩展 fileio 模式。

## 关键接口或数据结构

- `CompileRequest` / `RunRequest`：基础请求结构，承载 SubmissionID、WorkDir、Language、Profile、Limits 等通用字段。
- `CppCompileRequest` / `CppRunRequest`：C++ 扩展结构，当前实现直接复用基础字段，便于后续增加语言特定参数。
- `IOConfig`：I/O 模式（stdio / fileio）与文件名配置。
- `CheckerSpec`：SPJ 可执行文件与参数配置。
- `DefaultRunner`：默认实现，负责生成 RunSpec、调用 Engine，并映射 `CompileResult` / `TestcaseResult`。

## 使用示例或配置说明

1) 编译（C++）：
```
req := runner.CppCompileRequest{CompileRequest: runner.CompileRequest{
  SubmissionID: "sub-1",
  Language: langSpec,
  Profile: compileProfile,
  WorkDir: "/sandbox/sub-1/t1/work",
  SourcePath: "/sandbox/sub-1/source.cpp",
  ExtraCompileFlags: []string{"-O2"},
}}
res, err := r.CompileCpp(ctx, req)
```

2) 运行（C++ + SPJ）：
```
runReq := runner.CppRunRequest{RunRequest: runner.RunRequest{
  SubmissionID: "sub-1",
  TestID: "t1",
  Language: langSpec,
  Profile: runProfile,
  WorkDir: "/sandbox/sub-1/t1/work",
  IOConfig: runner.IOConfig{Mode: "stdio"},
  InputPath: "/sandbox/sub-1/t1/input.txt",
  AnswerPath: "/sandbox/sub-1/t1/answer.txt",
  Checker: &runner.CheckerSpec{BinaryPath: "/work/checker"},
  CheckerProfile: &checkerProfile,
}}
res, err := r.RunCpp(ctx, runReq)
```
