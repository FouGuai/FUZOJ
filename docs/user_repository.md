# User Repository

## 功能概览
该模块位于 `internal/user/repository`，负责用户与会话相关的数据访问层实现。它包含基于 PostgreSQL 的用户与 Token 仓储，基于 Redis 的用户缓存、封禁缓存与 Token 黑名单。用户仓储支持创建用户、按 ID/用户名/邮箱查询、检测用户名/邮箱存在性、更新密码与状态；Token 仓储支持创建、按 hash 查询、按 hash 撤销并加入黑名单、按用户批量撤销。缓存层提供读穿透（loader）能力与空值 TTL，降低缓存击穿与重复查询；封禁与黑名单以集合存储并根据过期时间自动延长 TTL，确保状态一致与成本可控。

## 关键接口或数据结构
- `User`、`UserStatus`、`UserRole`：用户核心模型与状态/角色枚举。
- `UserRepository`：用户持久化仓储接口（Postgres 实现）。
- `UserCacheRepository`：用户信息缓存接口，默认 TTL 30 分钟，空值 TTL 5 分钟。
- `UserToken`、`TokenType`：会话 Token 模型与类型枚举。
- `TokenRepository`：Token 持久化与黑名单管理接口（Postgres + Redis）。
- `BanCacheRepository`：用户封禁标记缓存接口。
- 缓存 Key：`user:info:`、`user:username:`、`user:email:`、`user:banned`、`token:blacklist`。

## 使用示例或配置说明
```go
userRepo := repository.NewUserRepository(db)
userCache := repository.NewUserCacheRepository(cacheClient)
tokenRepo := repository.NewTokenRepository(db, cacheClient)

user, err := userCache.GetByID(ctx, userID, func(ctx context.Context) (*repository.User, error) {
    return userRepo.GetByID(ctx, nil, userID)
})
if err != nil {
    return err
}

if err := tokenRepo.RevokeByUser(ctx, nil, user.ID); err != nil {
    return err
}
```
默认缓存 TTL 可通过 `NewUserCacheRepositoryWithTTL` 自定义；封禁与黑名单的 TTL 会根据传入的过期时间自动延长，不会缩短已有 TTL。
