# Sandbox Runner

## 功能概览

Sandbox Runner 是判题流程中的编排层，负责将语言配置、任务 Profile 与测试数据转化为 `RunSpec`，并调用 Sandbox Engine 执行编译、运行与 SPJ（交互题暂未实现但已预留扩展点）。当前实现按 `language_id` 分发到语言专属 runner：`CppRunner` 负责编译型 C++，`PythonRunner` 负责解释型 Python。Runner 采用统一的资源限制与判定规则：优先使用请求覆盖值，其次使用 Profile 默认限制，同时应用语言倍率（Time/Memory multiplier）。标准输入输出默认使用 stdio 模式，可扩展 fileio 模式。

## 关键接口或数据结构

- `CompileRequest` / `RunRequest`：基础请求结构，承载 SubmissionID、WorkDir、Language、Profile、Limits 等通用字段。
- `IOConfig`：I/O 模式（stdio / fileio）与文件名配置。
- `CheckerSpec`：SPJ 可执行文件与参数配置。
- `LanguageDispatchRunner`：统一入口，按 `language_id` 选择具体语言 runner。
- `CppRunner` / `PythonRunner`：语言专属实现；Python runner 会在每个测试目录写入源码后再执行解释器命令。

## 使用示例或配置说明

1) 编译（C++）：
``` 
req := runner.CompileRequest{
  SubmissionID: "sub-1",
  Language: langSpec,
  Profile: compileProfile,
  WorkDir: "/sandbox/sub-1/t1/work",
  SourcePath: "/sandbox/sub-1/source.cpp",
  ExtraCompileFlags: []string{"-O2"},
}
res, err := r.Compile(ctx, req)
```

2) 运行（Python）：
``` 
runReq := runner.RunRequest{
  SubmissionID: "sub-1",
  TestID: "t1",
  Language: langSpec,
  Profile: runProfile,
  WorkDir: "/sandbox/sub-1/t1/work",
  IOConfig: runner.IOConfig{Mode: "stdio"},
  InputPath: "/sandbox/sub-1/t1/input.txt",
  SourcePath: "/sandbox/sub-1/source.py",
}
res, err := r.Run(ctx, runReq)
```
