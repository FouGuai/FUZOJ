# 用户缓存仓库（简要指南）

**用途**：为用户模块提供的小型旁路缓存辅助工具，使缓存逻辑远离服务层。

**位置**：
- 代码位于 `internal/user/repository/user_cache.go`

**行为（高层级）**：
- `GetByID/GetByUsername/GetByEmail` 通过 `cache.GetWithCached` 使用旁路缓存模式
- `Set/Delete` 是用于写穿失效的辅助函数
- 空结果使用较短的 TTL 缓存，防止缓存穿透

**缓存键**：
- `user:info:{id}`
- `user:username:{username}`
- `user:email:{email}`

**TTL（生存时间）**：
- 默认数据 TTL：30 分钟
- 空值 TTL：5 分钟
- 可通过 `NewUserCacheRepositoryWithTTL` 覆盖配置

**注意**：本文档为简要指南；具体的接口和字段定义请查看代码实现。
