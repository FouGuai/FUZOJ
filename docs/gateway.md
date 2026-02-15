# Gateway 模块说明

## 1. 功能概览
Gateway 作为全站统一入口，提供鉴权、限流、黑名单/封禁校验与反向代理转发能力。设计目标是低延迟与高可用：请求在网关完成 JWT 自验证与快速失败，减少业务服务负载；封禁与黑名单检查遵循缓存优先原则，采用本地内存 + Redis 双层缓存；限流采用固定窗口计数器并支持按路由、用户与 IP 维度配置。网关支持通过配置文件定义路由与上游服务，便于扩展与灰度发布。

## 2. 关键接口与结构
- 配置：`configs/gateway.yaml`
  - `upstreams`：上游服务定义
  - `routes`：路由匹配、鉴权策略、限流与超时配置
  - `auth`：JWT secret/issuer
  - `cache`：封禁与黑名单本地缓存
- 核心组件：
  - AuthService：JWT 校验 + 黑名单 + 封禁检查
  - RateLimitService：Redis 计数限流
  - ProxyFactory：高性能反向代理与连接池复用
  - BanEventConsumer：订阅封禁事件，实时更新本地缓存

## 3. 使用与配置示例
- 新增路由：在 `routes` 中声明 `path/method/upstream/auth/ratelimit`，无需改代码。
- 鉴权策略：`public` 表示跳过鉴权；`protected` 表示必须携带有效 Token，可配置 `roles` 限制权限。
- 限流策略：全局默认值由 `rateLimit` 定义，单路由可覆盖。
