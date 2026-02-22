# User Auth Service

## 功能概览
用户模块已迁移至 go-zero 框架，代码位于 `services/user_service/`，入口为 `services/user_service/user.go`，配置为 `services/user_service/etc/user.yaml`。模块提供用户注册、登录、刷新令牌与登出能力，并包含用户、Token、封禁与黑名单等仓储能力。注册仅要求用户名与密码，服务端生成占位邮箱并使用 bcrypt 哈希；登录包含账号与 IP 维度的失败次数限制（15 分钟窗口）以防暴力破解。登录成功后签发 access/refresh JWT，并将 token 哈希写入数据库用于撤销与审计。刷新令牌采用轮换策略，旧 refresh 会被标记撤销并加入黑名单；登出仅撤销 refresh。用户读取与 Token 查询由 goctl 生成的 model 层缓存（`sqlc.CachedConn`）负责，缓存 Key 与失效策略由 go-zero 自动维护；黑名单与封禁列表使用 Redis 集合，确保快速判断与一致性。

## 关键接口或数据结构
- go-zero 分层：`internal/handler`（HTTP 入口）→ `internal/logic`（业务编排）→ `internal/repository`（数据访问）→ `internal/model`（goctl 生成模型）。
- `AuthLogic`：注册、登录、刷新、登出等核心业务逻辑。
- `RegisterRequest` / `LoginRequest` / `RefreshRequest` / `LogoutRequest`：HTTP 请求结构。
- `AuthResponse` / `UserInfo`：HTTP 响应结构。
- `UserRepository`：用户持久化仓储，基于 `UsersModel`（go-zero model 缓存）。
- `TokenRepository`：Token 持久化与黑名单管理，基于 `UserTokensModel` 与 Redis。
- `BanCacheRepository`：封禁标记缓存（Redis Set，Key: `user:banned`）。
- JWT Claims：包含 `sub`、`role`、`typ`、`iss`、`iat`、`exp`。

## 使用示例或配置说明
服务启动时需要注入 `JWTSecret` 与 TTL 配置，默认 access=15m、refresh=7d。登录失败计数键名为 `login:fail:username:{username}` 与 `login:fail:ip:{ip}`。用户与 Token 查询由 go-zero model 层缓存维护，缓存 key 示例：`cache:users:id:`、`cache:users:username:`、`cache:users:email:`、`cache:userTokens:tokenHash:`。黑名单集合为 `token:blacklist`，撤销 token 时会延长集合 TTL 以覆盖未过期 token。控制器通过统一响应结构返回 token 与基础用户信息，客户端可直接使用 `access_token` 与 `refresh_token` 建立会话。
