# User Auth Service

## 功能概览
该模块提供用户注册、登录、刷新令牌与登出能力，位于 `internal/user/service` 与 `internal/user/controller`。注册仅要求用户名与密码，服务端生成占位邮箱以满足现有数据约束，并使用 bcrypt 进行密码哈希。登录流程包含账号与 IP 维度的失败次数限制（15 分钟窗口），防止暴力破解；登录成功后签发 access/refresh JWT，并将 token 哈希写入数据库用于撤销与审计。用户读取通过 `UserRepository` 内部的 Cache-Aside 缓存完成，避免 Service 直接处理缓存细节。刷新令牌采用轮换策略，旧 refresh 会被标记撤销并加入黑名单；登出仅撤销 refresh。响应统一包含 token 与基础用户信息，便于客户端初始化会话。

## 关键接口或数据结构
- `AuthService`：注册、登录、刷新、登出等核心业务。
- `RegisterInput` / `LoginInput` / `RefreshInput` / `LogoutInput`：服务层输入模型。
- `AuthResult` / `UserInfo`：服务层输出模型。
- `AuthController`：HTTP 控制器，处理 `/api/v1/user/register`、`/api/v1/user/login`、`/api/v1/user/refresh-token`、`/api/v1/user/logout`。
- JWT Claims：包含 `sub`、`role`、`typ`、`iss`、`iat`、`exp`。

## 使用示例或配置说明
服务启动时需要注入 `JWTSecret` 与 TTL 配置，默认 access=15m、refresh=7d。登录失败计数键名为 `login:fail:username:{username}` 与 `login:fail:ip:{ip}`。控制器通过 `pkg/utils/response` 返回标准结构，客户端可直接使用 `access_token` 与 `refresh_token` 建立会话。
