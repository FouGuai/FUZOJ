# Sandbox 详细设计

## 1. 目标与范围

- 目标：为判题任务提供强隔离、可控资源、可观测的执行环境，支持多语言、多任务类型（编译/运行/SPJ/交互）。
- 范围：定义 Sandbox 引擎、配置模型、执行流程、I/O 组织、资源限制与安全策略，作为后续实现依据。

## 2. 总体设计

采用 **统一 Sandbox 引擎 + 多 Profile 配置** 的架构：

- **Sandbox Engine**：负责创建 namespace/cgroup、挂载文件、执行命令、采集资源统计、清理与回收。
- **Sandbox Profile**：按“语言 + 任务类型”定义 rootfs、seccomp、资源限制、命令模板等。
- **Task Runner**：编排编译/运行/SPJ/交互流程，生成 RunSpec，调用 Sandbox Engine。

这样既能做到“每种语言和任务都有独立 sandbox 规则”，又能复用核心执行逻辑，降低维护成本。

### 2.1 执行调用图与判题顺序


一次判题的调用顺序（简化）：

1. 接收判题请求，创建 `submissionId`
2. 拉取 `LanguageSpec` 与 `TaskProfile`（优先缓存，未命中回源）。
3. 准备工作目录 `/sandbox/{submissionId}/{testId}`，写入源码与测试数据。
4. 若 `compileEnabled=true`，生成编译 `RunSpec` 并调用 Sandbox Engine；失败则直接返回 CE。
5. 逐个测试点生成运行 `RunSpec` 并执行，采集时间/内存/输出。
6. 需要 SPJ/交互时，生成对应 `RunSpec` 并执行比较器或交互器。
7. 汇总所有测试点结果，计算最终 `verdict/score/summary`。
8. 清理 sandbox 目录与 cgroup

更新状态由上层服务更新，sandbox就是无状态的执行器，只会记录一些自己的性能信息

### 2.2 Runner 实现要点（当前实现）

Runner 负责将语言与任务配置转化为可执行的 `RunSpec`，并调用 Sandbox Engine。当前实现以 **C++ 为优先支持语言**，并通过“基础请求结构 + 语言扩展结构”的方式预留多语言扩展：

- `CompileRequest` / `RunRequest` 作为基础结构，承载通用字段（SubmissionID、WorkDir、语言配置、Profile、资源限制等）。
- 语言扩展采用结构体嵌入（如 `CppCompileRequest` / `CppRunRequest`），便于后续横向扩展到其它语言。

Runner 生成 RunSpec 的关键规则：

1. `Cmd` 由 `LanguageSpec.CompileCmdTpl` / `RunCmdTpl` 生成，替换 `{src}`、`{bin}`、`{extraFlags}`，并用 `strings.Fields` 分词。
2. `WorkDir` 固定为容器路径 `/work`，通过 bind mount 把主机工作目录挂载到容器。
3. `BindMounts` 包含工作目录（读写）与输入/答案（只读）。
4. 资源限制优先使用请求中覆盖值，其次使用 `TaskProfile.DefaultLimits`，并应用语言倍率（Time/Memory multiplier）。
5. 编译日志写入 `/work/compile.log`，运行日志写入 `/work/runtime.log`。

#### SPJ（Checker）流程

当判题需要 SPJ 时，Runner 会生成独立的 Checker RunSpec，命令为：

```
checker input.txt output.txt answer.txt
```

Checker 运行结果用于判断 AC/WA；其日志写入 `/work/checker.log`。交互题流程暂不实现，但 Runner 结构已预留扩展点。

## 3. 执行目录与文件模型

每个测试点独立目录：

```
/sandbox/{submissionId}/{testId}/
  /work/                    # 读写工作目录
    main.cpp                # 源码
    main                    # 编译产物
    input.txt               # 测试输入（只读）
    output.txt              # 选手输出（可写）
    answer.txt              # 标准答案（只读）
    compile.log             # 编译日志
    runtime.log             # 运行 stderr
```

- rootfs 只读挂载；`/work` 读写挂载。
- 输入与答案文件只读 bind mount。
- 输出文件配合 RLIMIT_FSIZE 与监控双重限制。

## 4. 资源限制与安全隔离

### 4.1 隔离机制

- **namespaces**：`pid`, `mount`, `user`, `uts`, `ipc`, `net`（禁用网络）。
- **cgroups v2**：CPU、Memory、PIDs、IO 限制。
- **seccomp**：白名单系统调用（按语言/任务类型区分）。
- **rlimits**：`RLIMIT_CPU`、`RLIMIT_FSIZE`、`RLIMIT_STACK`、`RLIMIT_NPROC`。

### 4.2 资源限制字段（建议统一）

```
ResourceLimit {
  cpuTimeMs: int        # 纯 CPU 时间
  wallTimeMs: int       # 墙钟时间
  memoryMB: int         # RSS 上限
  stackMB: int          # 栈上限
  outputMB: int         # 输出上限
  pids: int             # 进程数上限
}
```

- CPU / wall time 双重限制；任一超限判定 TLE。
- Memory 超限判定 MLE。
- Output 超限判定 OLE。

## 5. RunSpec 统一执行规格

建议用统一结构承载执行所需信息：

```
RunSpec {
  workDir: string
  cmd: []string
  env: []string
  stdinPath: string
  stdoutPath: string
  stderrPath: string
  bindMounts: []MountSpec
  profile: string            # seccomp/rootfs/rlimits 选择
  limits: ResourceLimit
}
```

## 6. 配置模型（多语言支持）

### 6.1 LanguageSpec

```
LanguageSpec {
  id: "cpp"
  name: "C++"
  version: "gnu++17"
  sourceFile: "main.cpp"
  binaryFile: "main"
  compileEnabled: true
  compileCmdTpl: "g++ -std=gnu++17 -O2 -pipe {extraFlags} -o {bin} {src}"
  runCmdTpl: "{bin}"
  env: ["LANG=C", "LC_ALL=C"]
  timeMultiplier: 1.0
  memoryMultiplier: 1.0
}
```

### 6.2 TaskProfile

```
TaskProfile {
  taskType: "compile" | "run" | "checker" | "interactor"
  rootfs: "cpp-compile-rootfs" | "cpp-run-rootfs" | ...
  seccompProfile: "cpp-compile.json"
  defaultLimits: ResourceLimit
}
```

### 6.3 配置来源与缓存

- LanguageSpec / TaskProfile 存储于 DB，加载后写入 Redis（TTL 30min）。
- 变更时触发本地缓存刷新（本地 LRU + TTL）。

### 6.4 无编译语言支持（解释型/脚本）

设计原则：**编译步骤可选**，由 `compileEnabled` 控制。

- `compileEnabled=false` 时：跳过编译阶段，直接进入运行阶段。
- `compileCmdTpl` 允许为空；`runCmdTpl` 使用解释器运行：
  - 例如 Python：`runCmdTpl: "python3 {src}"`
  - 例如 JavaScript：`runCmdTpl: "node {src}"`
- 如果需要预编译（如 Java/Kotlin）：`compileEnabled=true`，`compileCmdTpl` 输出字节码，`runCmdTpl` 执行 VM。
- 脚本语言允许 **语法检查可选步骤**（profile=lint），但默认不强制。

## 7. 编译流程（C++ 示例）

### 7.1 编译命令模板

```
compileCmd = g++ -std=gnu++17 -O2 -pipe {extraFlags} -o /work/main /work/main.cpp
```

- 编译输出重定向至 `/work/compile.log`。
- 编译超时建议 5s，内存 1GB。

### 7.2 编译流程

1. 生成编译 RunSpec（profile=cpp-compile）。
2. 写入源代码到 `/work/main.cpp`。
3. 执行编译命令，采集 exit code + log。
4. 编译失败：判定 CE，保存 compile.log。


### 7.4 无编译语言执行流程

1. `compileEnabled=false` 时直接进入运行阶段。
2. 生成运行 RunSpec（profile=language-run）。
3. 运行命令一般为解释器或 VM：`python3 {src}` / `node {src}`。
4. 返回结果不包含编译产物字段，但仍需返回运行资源与日志路径。

## 8. 运行流程

### 8.1 标准 IO 模式（默认）

```
/work/main < /work/input.txt > /work/output.txt 2> /work/runtime.log
```

- `input.txt` 为当前测试点输入。
- `output.txt` 作为选手输出。

### 8.2 file I/O 模式（支持）

- 题目配置指定 `ioMode=fileio` 与 `inputFileName/outputFileName`。
- sandbox 在 `/work` 生成 `input.txt`，要求程序读写固定文件名。
- 不再做 stdin/stdout 重定向。

## 9. SPJ / 交互题支持

### 9.1 SPJ

- 选手程序输出 `output.txt`。
- checker 在独立 sandbox 执行：

```
checker /work/input.txt /work/output.txt /work/answer.txt
```

- checker 只读 `input/answer`，读写自身 stdout/stderr。

### 9.2 交互题

- 选手程序与 interactor 同时运行。
- 通过双向管道（pipe/pty）连接。
- interactor 负责输入输出协议校验。
- 独立 wall time 与输出限制。

## 10. 自定义编译选项

### 10.1 允许的选项白名单

- `-O0/-O1/-O2/-O3/-Ofast`
- `-std=gnu++17`（可允许更高版本按配置）
- `-static`（允许，要求镜像含静态库）

### 10.2 禁止的选项

- `-shared`, `-fPIC`, `-Winvalid-pch`
- 自定义 `-o`（避免覆盖产物路径）
- 自定义 `-I` 需要题目白名单授权

### 10.3 过滤流程

1. 接收 `extraFlags`。
2. 词法拆分，逐项匹配白名单。
3. 命中黑名单直接拒绝。
4. 过滤后拼接进入 `compileCmdTpl`。

## 11. seccomp 规则管理

- 使用 **按语言/任务类型区分** 的白名单。
- 编译与运行分离（编译需要更多 syscalls）。
- 推荐维护规则仓库，例如：`configs/seccomp/cpp-compile.json`。

## 12. killTask 设计

- 以 `submissionId` 维度记录 cgroup 路径。
- killTask：对 cgroup 内进程组发送 `SIGKILL`。
- 清理相关 `/sandbox/{submissionId}` 目录。

## 13. 结果映射与优先级

优先级建议：

1. System Error（sandbox 失败）
2. TLE（wall/cpu 超时）
3. MLE（memory 超限）
4. OLE（output 超限）
5. RE（非 0 退出码）
6. WA/AC（比较器或 SPJ 结果）

## 14. 结果数据格式（统一返回结构）

### 14.1 顶层返回结构（建议 JSON）
```
JudgeResult {
  submissionId: string
  status: "Pending" | "Running" | "Finished" | "Failed"
  verdict: "AC" | "WA" | "TLE" | "MLE" | "OLE" | "RE" | "CE" | "SE"
  score: int
  language: string
  compile: CompileResult | null
  tests: []TestcaseResult
  summary: SummaryStat
  timestamps: {
    receivedAt: int64
    finishedAt: int64
  }
}
```

### 14.2 编译结果（compileEnabled=true）
```
CompileResult {
  ok: bool
  exitCode: int
  timeMs: int
  memoryKB: int
  logPath: string
  error: string
}
```

### 14.3 测试点结果
```
TestcaseResult {
  testId: string
  verdict: "AC" | "WA" | "TLE" | "MLE" | "OLE" | "RE"
  timeMs: int
  memoryKB: int
  outputKB: int
  exitCode: int
  runtimeLogPath: string
  checkerLogPath: string
  stdout: string
  stderr: string
  score: int
  subtaskId: string
}
```

### 14.4 汇总统计
```
SummaryStat {
  totalTimeMs: int
  maxMemoryKB: int
  totalScore: int
  failedTestId: string
}
```

### 14.5 无编译语言返回要求
- `compile` 字段为 `null`。
- `verdict` 由测试点结果聚合得到。
- 运行日志字段仍必须返回。

## 15. 可观测性与日志

- 记录：编译耗时、运行耗时、内存峰值、输出大小、exit code。
- 结构化日志必须使用英文内容。
- Metrics：按语言/题目/任务统计失败率与耗时分位。

## 16. C++ 语言落地配置示例

```
LanguageSpec:
  id: cpp
  version: gnu++17
  compileCmdTpl: g++ -std=gnu++17 -O2 -pipe {extraFlags} -o {bin} {src}
  runCmdTpl: {bin}

TaskProfile:
  compile:
    rootfs: cpp-compile-rootfs
    seccomp: cpp-compile.json
    limits: { cpuTimeMs: 5000, wallTimeMs: 7000, memoryMB: 1024, outputMB: 64, pids: 64 }
  run:
    rootfs: cpp-run-rootfs
    seccomp: cpp-run.json
    limits: { cpuTimeMs: TL, wallTimeMs: WL, memoryMB: ML, outputMB: OL, pids: 32 }
```

## 17. 扩展方向

- 新语言：新增 LanguageSpec + TaskProfile + rootfs + seccomp，即可接入。
- 新任务类型：新增 taskType 与对应 Profile。
- 多层缓存：编译产物、SPJ 二进制、测试数据包优先本地缓存，未命中再走 Redis/MinIO。
