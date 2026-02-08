# User Token Repository

## 功能概览

该模块位于 `internal/user/repository/session.go`，负责用户 Token 的持久化与黑名单管理。Token 哈希存储在 mySQL，支持按 hash 查询、单个撤销、按用户批量撤销；撤销操作会将 token hash 写入 Redis 的 `token:blacklist` 集合，并根据 token 过期时间延长集合 TTL，确保黑名单生效时间覆盖所有未过期的 token。`TokenRepository` 内部实现 Cache-Aside 缓存，缓存 TTL 会基于 token 过期时间并加入随机抖动，降低缓存雪崩风险。该设计与登录/刷新/登出流程协同，用于快速校验 token 是否已被撤销。

## 关键接口或数据结构

- `UserToken`：token 数据模型，包含 `token_hash`、`token_type`、`expires_at` 等字段。
- `TokenRepository`：核心接口，提供 `Create`、`GetByHash`、`RevokeByHash`、`RevokeByUser`、`IsBlacklisted`。
- `PostgresTokenRepository`：MySQL + Redis 实现，写库 + 黑名单集合 + Token 缓存。
- Redis Key：`token:blacklist`（Set）。

## 使用示例或配置说明

```go
repo := repository.NewTokenRepository(db, cacheClient)

// 记录 token
_ = repo.Create(ctx, tx, &repository.UserToken{
    UserID:    userID,
    TokenHash: tokenHash,
    TokenType: repository.TokenTypeAccess,
    ExpiresAt: expiresAt,
})

// 撤销 token 并写入黑名单
_ = repo.RevokeByHash(ctx, tx, tokenHash, expiresAt)
```

`RevokeByHash`/`RevokeByUser` 会将 token hash 写入 `token:blacklist`，并根据过期时间延长集合 TTL，避免黑名单提前失效。Token 缓存 TTL 会基于过期时间并加入随机抖动，避免集中失效。
