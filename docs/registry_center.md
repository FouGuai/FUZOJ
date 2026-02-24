# 注册中心与配置中心（Etcd）格式说明

## 功能概览
本文件定义服务在注册中心的注册格式与运行时配置、日志配置的存储规范。服务启动时从 Etcd 读取运行时与日志配置，再注册自身地址到 Etcd。该规范用于保证多实例可用、配置中心统一与网关后续路由扩展的一致性。

## 关键结构与 Key 约定
1) **运行时配置（REST）**  
Key：`{service}.rest.runtime`  
Value：JSON，对应 `rest.RestConf` 的最小启动字段：  
`name`、`host`、`port`、`timeout`、`middlewares`，以及可选 `registerKey`。  
`registerKey` 默认是 `{service}.rest`。

2) **运行时配置（RPC）**  
Key：`{service}.rpc.runtime`  
Value：JSON，对应 `zrpc.RpcServerConf` 的最小启动字段：  
`listenOn` 与 `etcd`（含 `hosts`、`key`）。

3) **日志配置**  
Key：`{service}.log`  
Value：JSON。  
Gateway 使用 `pkg/utils/logger.Config`；其余服务使用 `logx.LogConf`。

4) **服务注册（REST）**  
Key：`{service}.rest`（或 `registerKey` 指定）  
Value：`host:port`  
多实例通过 Etcd 租约自动生成子 Key（如 `key/{leaseId}`），无需手动维护实例 ID。

## 使用示例
REST 运行时（`user.rest.runtime`）：
```json
{"name":"user","host":"0.0.0.0","port":8081,"timeout":"0s","middlewares":{"recover":true},"registerKey":"user.rest"}
```
RPC 运行时（`problem.rpc.runtime`）：
```json
{"listenOn":"0.0.0.0:9093","etcd":{"hosts":["127.0.0.1:2379"],"key":"problem.rpc"}}
```
日志（`user.log`）：
```json
{"serviceName":"user","mode":"console","encoding":"json","level":"info"}
```

## 更新约定
如新增或变更 Etcd 中的运行时字段、日志字段、Key 命名规则，必须同步更新本文件，保持与代码实现一致。
