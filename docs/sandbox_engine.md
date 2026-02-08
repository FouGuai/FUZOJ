# Sandbox Engine

## 功能概览
Sandbox Engine 提供 Linux 原生沙箱执行能力，负责在每次运行前创建 cgroup、初始化资源限制与隔离策略，并在受控环境中执行命令。引擎通过 profile 解析 rootfs、seccomp 规则与网络隔离开关，按 RunSpec 执行编译或运行任务，采集 CPU 时间、墙钟时间、内存峰值与输出大小，并在结束后清理 cgroup。引擎内部采用 per-run cgroup，KillSubmission 会批量终止同一 submission 下的全部运行实例。

## 关键接口或数据结构
- `spec.RunSpec`：新增 `SubmissionID/TestID` 用于 cgroup 命名与 KillSubmission 归属；`Limits` 描述 CPU/Wall/Memory/Stack/Output/PIDs 上限。
- `result.RunResult`：`TimeMs` 为 CPU 时间（用户态+系统态），`WallTimeMs` 为墙钟时间（内部使用），`MemoryKB` 优先取 cgroup `memory.peak`，`OutputKB` 仅统计 stdout。
- `engine.Config`：包含 `CgroupRoot`、`SeccompDir`、`HelperPath`、`StdoutStderrMaxBytes`、`EnableSeccomp/EnableCgroup/EnableNamespaces`。
- `ProfileResolver`：将 `RunSpec.Profile` 解析为 `security.IsolationProfile`（RootFS、SeccompProfile、DisableNetwork）。
- `cmd/sandbox-init`：沙箱初始化器，读取 JSON 请求，完成 bind mount、chroot、rlimits、seccomp，并 `exec` 目标命令。

## 使用示例或配置说明
1) 引擎初始化：
```
resolver := YourProfileResolver{}
cfg := engine.Config{
  CgroupRoot: "/sys/fs/cgroup/fuzoj",
  SeccompDir: "/etc/fuzoj/seccomp",
  HelperPath: "sandbox-init",
  StdoutStderrMaxBytes: 64 * 1024,
  EnableSeccomp: true,
  EnableCgroup: true,
  EnableNamespaces: true,
}
eng, _ := engine.NewEngine(cfg, resolver)
```
2) 运行规格：
```
run := spec.RunSpec{
  SubmissionID: "sub-1",
  TestID: "t1",
  WorkDir: "/work",
  Cmd: []string{"/work/main"},
  StdoutPath: "/work/output.txt",
  StderrPath: "/work/runtime.log",
  Profile: "cpp-run",
  Limits: spec.ResourceLimit{CPUTimeMs: 1000, WallTimeMs: 2000, MemoryMB: 256, OutputMB: 64, PIDs: 32},
}
```
3) 运行结果采集：
`TimeMs` 为 CPU 时间；`WallTimeMs` 用于内部超时判定与监控；`MemoryKB` 使用 `memory.peak`，回退 `rusage.Maxrss`；`OutputKB` 只统计 stdout 文件大小。
