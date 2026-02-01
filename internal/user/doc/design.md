# FuzOJ 用户模块架构设计文档

## 1. 整体架构设计

### 1.1 核心设计理念

**问题背景**：
- 高并发场景下，每次请求都访问用户服务会成为性能瓶颈
- 需要支持万级并发的用户鉴权
- 用户封禁需要实时生效且持久化管理

**解决方案**：
```
┌──────────────────────────────────────────────────────────────┐
│                      Client Request                          │
└──────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌──────────────────────────────────────────────────────────────┐
│                   Gateway (鉴权中间件)                        │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  1. JWT 自验证（无需调用用户服务）                       │    │
│  │  2. Redis 检查 Token 黑名单（短期失效）                 │    │
│  │  3. Redis 检查用户封禁状态（长期封禁）                   │    │
│  │  4. 注入 user_id 到请求上下文                          │    │
│  └─────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────┘
                            │
            ┌───────────────┴───────────────┐
            ▼                               ▼
┌─────────────────────┐         ┌─────────────────────┐
│  Business Services  │         │   User Service      │
│  (Judge/Contest)    │         │  (仅低频操作)         │
│                     │         │  - 注册/登录          │
│  直接使用 user_id    │         │   - 修改密码         │
│  无需调用用户服务     │          │  - 封禁管理          │
└─────────────────────┘         └─────────────────────┘
```

### 1.2 技术选型

- **JWT**: 无状态鉴权，减少服务调用
- **Redis**: 高性能缓存和黑名单存储
- **MySQL**: 持久化用户数据和封禁记录
- **Bcrypt**: 密码加密算法
- **Snowflake**: 分布式 ID 生成（可选）

---

## 2. 数据库设计

### 2.1 用户表 (users)

```sql
CREATE TABLE `users` (
    `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '用户唯一ID',
    `username` VARCHAR(32) NOT NULL UNIQUE COMMENT '用户名',
    `email` VARCHAR(128) NOT NULL UNIQUE COMMENT '邮箱',
    `phone` VARCHAR(20) DEFAULT NULL UNIQUE COMMENT '手机号（预留）',
    `password_hash` VARCHAR(128) NOT NULL COMMENT 'Bcrypt Hash',
    `salt` VARCHAR(32) NOT NULL COMMENT '密码盐',
    `role` ENUM('guest', 'user', 'problem_setter', 'contest_manager', 'admin', 'super_admin') 
           DEFAULT 'user' COMMENT '用户角色',
    `status` ENUM('active', 'banned', 'pending_verify') 
             DEFAULT 'pending_verify' COMMENT '账号状态',
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX `idx_email` (`email`),
    INDEX `idx_username` (`username`),
    INDEX `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户基础表';
```

### 2.2 用户封禁表 (user_bans) ⭐ 核心改进

```sql
CREATE TABLE `user_bans` (
    `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '封禁记录ID',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT '被封禁用户ID',
    `ban_type` ENUM('permanent', 'temporary') DEFAULT 'permanent' COMMENT '封禁类型',
    `reason` TEXT NOT NULL COMMENT '封禁原因',
    `banned_by` BIGINT UNSIGNED NOT NULL COMMENT '操作管理员ID',
    `start_time` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '封禁开始时间',
    `end_time` DATETIME DEFAULT NULL COMMENT '解封时间（NULL表示永久）',
    `status` ENUM('active', 'expired', 'cancelled') DEFAULT 'active' COMMENT '封禁状态',
    `cancel_reason` TEXT DEFAULT NULL COMMENT '取消封禁原因',
    `cancelled_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '取消封禁的管理员ID',
    `cancelled_at` DATETIME DEFAULT NULL COMMENT '取消封禁时间',
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX `idx_user_id` (`user_id`),
    INDEX `idx_status` (`status`),
    INDEX `idx_end_time` (`end_time`)
) COMMENT='用户封禁记录表';
```

**设计要点**：
- `ban_type`: 区分永久和临时封禁
- `end_time`: NULL 表示永久封禁，有值表示到期自动解封
- `status`: active（生效中）/ expired（已过期）/ cancelled（已取消）
- 记录完整的封禁历史和操作者，便于审计

### 2.3 Token 记录表 (user_tokens)

```sql
CREATE TABLE `user_tokens` (
    `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    `user_id` BIGINT UNSIGNED NOT NULL,
    `token_hash` VARCHAR(64) NOT NULL UNIQUE COMMENT 'Token SHA256',
    `token_type` ENUM('access', 'refresh') DEFAULT 'access',
    `device_info` VARCHAR(256) DEFAULT NULL COMMENT '设备信息',
    `ip_address` VARCHAR(45) DEFAULT NULL,
    `expires_at` DATETIME NOT NULL,
    `revoked` TINYINT(1) DEFAULT 0 COMMENT '是否已撤销',
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX `idx_user_id` (`user_id`),
    INDEX `idx_token_hash` (`token_hash`),
    INDEX `idx_expires_at` (`expires_at`)
) COMMENT='Token记录表（用于多设备管理）';
```

---

## 3. Redis 缓存设计

### 3.1 数据结构定义

```
# 1. 用户基础信息缓存（Hash）
Key: user:info:{user_id}
Value: {
    "id": 10001,
    "username": "alice",
    "email": "alice@fuzoj.com",
    "role": "user",
    "status": "active"
}
TTL: 1800s (30分钟)

# 2. 用户封禁黑名单（Set）
Key: user:banned
Value: Set<user_id>  # 只存储当前被封禁的 user_id
示例: {10001, 10002, 10003}
TTL: 永久（通过数据库同步）

# 3. Token 撤销黑名单（Set）
Key: token:blacklist
Value: Set<token_hash>  # SHA256(token)
TTL: 按 Token 过期时间设置

# 4. 登录失败限流（String） # 登陆失败超过五次加入这个
Key: login:fail:{identifier}  # identifier = username/email/ip
Value: fail_count
TTL: 900s (15分钟)
```

### 3.2 Redis 与 MySQL 同步策略

```go
// 伪代码：封禁用户时的数据同步
func BanUser(userID int64, reason string, endTime *time.Time) error {
    // 1. 写入 MySQL
    ban := &UserBan{
        UserID:    userID,
        BanType:   if endTime == nil ? "permanent" : "temporary",
        Reason:    reason,
        EndTime:   endTime,
        Status:    "active",
    }
    db.Create(ban)
    
    // 2. 更新用户状态
    db.Model(&User{}).Where("id = ?", userID).Update("status", "banned")
    
    // 3. 同步到 Redis
    redis.SAdd("user:banned", userID)
    
    // 4. 撤销该用户所有 Token
    tokens := db.Where("user_id = ? AND revoked = 0", userID).Find(&UserToken{})
    for _, token := range tokens {
        redis.SAdd("token:blacklist", token.TokenHash)
        redis.Expire("token:blacklist", token.ExpiresAt)
    }
    db.Model(&UserToken{}).Where("user_id = ?", userID).Update("revoked", 1)
    
    // 5. 清除用户信息缓存
    redis.Del(fmt.Sprintf("user:info:%d", userID))
    
    return nil
}
```

```go
// 伪代码：解封用户
func UnbanUser(userID int64, cancelReason string, operatorID int64) error {
    // 1. 更新封禁记录
    db.Model(&UserBan{}).
        Where("user_id = ? AND status = 'active'", userID).
        Updates(map[string]interface{}{
            "status": "cancelled",
            "cancel_reason": cancelReason,
            "cancelled_by": operatorID,
            "cancelled_at": time.Now(),
        })
    
    // 2. 更新用户状态
    db.Model(&User{}).Where("id = ?", userID).Update("status", "active")
    
    // 3. 从 Redis 黑名单移除
    redis.SRem("user:banned", userID)
    
    // 4. 清除用户信息缓存（让其重新加载）
    redis.Del(fmt.Sprintf("user:info:%d", userID))
    
    return nil
}
```

---

## 4. JWT Token 设计

### 4.1 Token 结构

```go
// Access Token (30分钟)
type AccessTokenClaims struct {
    UserID   int64  `json:"uid"`      // 用户ID（核心标识）
    Username string `json:"username"` // 用户名
    Role     string `json:"role"`     // 角色
    jwt.RegisteredClaims
}

// Refresh Token (7天)
type RefreshTokenClaims struct {
    UserID int64 `json:"uid"`
    jwt.RegisteredClaims
}
```

### 4.2 Token 签发流程

```go
func Login(identifier, password string) (accessToken, refreshToken string, err error) {
    // 1. 验证用户凭证
    user := db.Where("username = ? OR email = ?", identifier, identifier).First(&User{})
    if !bcrypt.Compare(user.PasswordHash, password) {
        return "", "", errors.New("invalid credentials")
    }
    
    // 2. 检查用户状态
    if user.Status == "banned" {
        return "", "", errors.New("user is banned")
    }
    
    // 3. 生成 Token
    accessToken = jwt.Sign(AccessTokenClaims{
        UserID:   user.ID,
        Username: user.Username,
        Role:     user.Role,
        ExpiresAt: time.Now().Add(30 * time.Minute),
    })
    
    refreshToken = jwt.Sign(RefreshTokenClaims{
        UserID:    user.ID,
        ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
    })
    
    // 4. 记录 Token
    db.Create(&UserToken{
        UserID:    user.ID,
        TokenHash: sha256(accessToken),
        TokenType: "access",
        ExpiresAt: time.Now().Add(30 * time.Minute),
    })
    
    // 5. 缓存用户信息
    redis.HSet(fmt.Sprintf("user:info:%d", user.ID), user)
    redis.Expire(fmt.Sprintf("user:info:%d", user.ID), 30*time.Minute)
    
    return accessToken, refreshToken, nil
}
```

---

## 5. Gateway 鉴权中间件

### 5.1 核心流程

```go
func AuthMiddleware(c *gin.Context) {
    token := extractToken(c.GetHeader("Authorization"))
    
    // 步骤1: JWT 自验证（不访问数据库/Redis）
    claims, err := jwt.Parse(token)
    if err != nil {
        c.AbortWithStatusJSON(401, gin.H{"error": "Invalid token"})
        return
    }
    
    // 步骤2: 检查 Token 是否被撤销（Redis）
    tokenHash := sha256(token)
    if redis.SIsMember("token:blacklist", tokenHash) {
        c.AbortWithStatusJSON(401, gin.H{"error": "Token revoked"})
        return
    }
    
    // 步骤3:  检查用户是否被封禁（Redis）⭐ 基于 user_id  （如果用户大的话要更新）
    if redis.SIsMember("user:banned", claims.UserID) {
        c.AbortWithStatusJSON(403, gin.H{"error": "User banned"})
        return
    }
    
    // 步骤4: 注入用户信息到上下文
    c.Set("user_id", claims.UserID)
    c.Set("username", claims.Username)
    c.Set("role", claims.Role)
    
    c.Next()
}
```

**性能分析**：
- JWT 验证：纯内存操作，耗时 < 0.1ms
- Redis 检查：单次查询 < 1ms
- **总耗时 < 2ms，支持万级 QPS**

### 5.2 Redis 故障时的降级策略（高可用）

```go
// 伪代码：带降级保护的鉴权中间件
func AuthMiddlewareWithFallback(c *gin.Context) {
    token := extractToken(c.GetHeader("Authorization"))
    claims, err := jwt.Parse(token)
    if err != nil {
        c.AbortWithStatusJSON(401, gin.H{"error": "Invalid token"})
        return
    }
    
    // 步骤1: 尝试检查 Redis
    isBanned, redisErr := checkUserBannedRedis(claims.UserID)
    
    // 步骤2: Redis 故障时降级策略
    if redisErr != nil {
        // 降级方案1：本地缓存（最快）
        isBanned = checkLocalCache(claims.UserID)
        
        // 降级方案2：如果本地缓存也没有，再查数据库（有限流保护）
        if !isBanned && shouldQueryDB() {
            isBanned, _ = checkUserBannedDB(claims.UserID)
        }
        
        // 降级方案3：如果数据库连接池满，则拒绝请求（保护数据库）
        if !shouldQueryDB() {
            c.AbortWithStatusJSON(503, gin.H{
                "error": "Service temporarily unavailable",
                "reason": "Database overload",
            })
            return
        }
    }
    
    if isBanned {
        c.AbortWithStatusJSON(403, gin.H{"error": "User banned"})
        return
    }
    
    c.Set("user_id", claims.UserID)
    c.Next()
}
```

#### 降级方案详解

**方案1：本地缓存（优先级最高）**

```go
type LocalBanCache struct {
    cache  map[int64]bool
    mu     sync.RWMutex
    ttl    time.Duration
}

// 启动时预热缓存
func (c *LocalBanCache) Init() {
    // 从数据库加载所有被封禁用户
    bans := db.Where("status = 'active'").Find(&UserBan{})
    for _, ban := range bans {
        c.cache[ban.UserID] = true
    }
    log.Infof("Loaded %d banned users to local cache", len(bans))
    
    // 定期同步（每 10 秒）
    go c.syncFromDB()
}

func (c *LocalBanCache) syncFromDB() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        // 从 Redis 同步增量变化
        // 如果 Redis 也故障，则定期查询数据库
        c.updateFromRedis()
    }
}

func (c *LocalBanCache) Check(userID int64) bool {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.cache[userID]
}
```

**优势**：
- 响应时间 < 0.01ms（内存查询）
- Redis 故障时也能继续工作
- 不给数据库增加任何压力

**局限**：
- 缓存不是实时的（最多延迟 10 秒）
- 新的封禁可能延迟生效

**适用场景**：
- Redis 故障但未完全宕机
- 可以接受 10 秒内的延迟

---

**方案2：限流保护的数据库查询**

```go
type FallbackQueryLimiter struct {
    concurrency int        // 最多同时并发查询数
    activeCount atomic.Int // 当前进行中的查询数
    maxQueueSize int       // 队列最大长度
    queue chan func()      // 查询队列
}

func (l *FallbackQueryLimiter) Query(userID int64) (bool, error) {
    // 检查是否超过并发限制
    if l.activeCount.Load() >= int32(l.concurrency) {
        // 拒绝请求，保护数据库
        return false, errors.New("database overload")
    }
    
    l.activeCount.Add(1)
    defer l.activeCount.Add(-1)
    
    // 带超时的数据库查询
    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()
    
    var ban UserBan
    err := db.WithContext(ctx).
        Where("user_id = ? AND status = 'active'", userID).
        First(&ban).Error
    
    if err == context.DeadlineExceeded {
        // 查询超时，说明数据库压力很大，降级失败
        return false, errors.New("database query timeout")
    }
    
    return ban.ID != 0, nil
}

// 启动时创建限流器
fallbackLimiter := &FallbackQueryLimiter{
    concurrency: 20,       // 最多 20 个并发
    maxQueueSize: 1000,
}
```

**降级决策树**：

```
请求来到 Gateway
    ↓
1. 尝试 Redis 查询
    ├─ 成功 → 返回结果 ✅
    └─ 失败 → 进入降级
    
2. 降级第一步：查询本地缓存
    ├─ 缓存命中 → 返回结果 ✅
    └─ 缓存未命中 → 进入降级
    
3. 降级第二步：限流保护的数据库查询
    ├─ 并发 < 20 → 查询数据库（最多等 100ms）
    │   ├─ 成功 → 返回结果 ✅
    │   └─ 超时 → 进入降级
    └─ 并发 ≥ 20 → 拒绝请求
    
4. 降级第三步：快速失败（保护系统）
    ├─ 返回 503 Service Unavailable
    └─ 用户重试或等待系统恢复
```

**方案3：断路器模式**

```go
type CircuitBreaker struct {
    state           string  // "closed", "open", "half-open"
    failureCount    int
    failureThreshold int   // 失败多少次后断路
    successThreshold int   // 成功多少次后恢复
    lastFailureTime time.Time
    halfOpenTimeout time.Duration
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    // 检查断路器状态
    if cb.state == "open" {
        // 检查是否可以尝试恢复
        if time.Since(cb.lastFailureTime) > cb.halfOpenTimeout {
            cb.state = "half-open"
            cb.failureCount = 0
        } else {
            return errors.New("circuit breaker open")
        }
    }
    
    // 执行函数
    err := fn()
    
    if err != nil {
        cb.failureCount++
        cb.lastFailureTime = time.Now()
        
        // 失败次数过多，打开断路器
        if cb.failureCount >= cb.failureThreshold {
            cb.state = "open"
            log.Warn("Circuit breaker opened due to too many failures")
        }
        return err
    }
    
    // 成功，重置计数
    if cb.state == "half-open" {
        cb.failureCount++
        if cb.failureCount >= cb.successThreshold {
            cb.state = "closed"
            cb.failureCount = 0
            log.Info("Circuit breaker closed, service recovered")
        }
    } else {
        cb.failureCount = 0
    }
    
    return nil
}

// 使用示例
redisBreaker := &CircuitBreaker{
    state: "closed",
    failureThreshold: 10,  // 失败 10 次打开
    successThreshold: 5,   // 成功 5 次关闭
    halfOpenTimeout: 30 * time.Second,
}

func checkUserBannedWithCircuitBreaker(userID int64) bool {
    // Redis 查询
    err := redisBreaker.Call(func() error {
        _, err := redis.SIsMember("user:banned", userID)
        return err
    })
    
    if err != nil {
        // Redis 故障，降级到本地缓存
        return localCache.Check(userID)
    }
    
    return false
}
```

**断路器的三个状态**：

```
Closed (正常)
    ├─ 失败次数 < 10 → 继续工作
    └─ 失败次数 ≥ 10 → 转为 Open
    
Open (故障)
    └─ 等待 30 秒后 → 转为 Half-Open
    
Half-Open (恢复中)
    ├─ 成功 5 次 → 转为 Closed (完全恢复)
    └─ 再次失败 → 转为 Open (继续故障)
```

---

#### 完整的降级配置

```yaml
# config.yaml
fallback:
  # 本地缓存
  local_cache:
    enabled: true
    max_size: 100000
    sync_interval: 10s
    
  # 数据库限流
  db_query:
    enabled: true
    max_concurrent: 20      # 最多 20 个并发查询
    query_timeout: 100ms    # 查询超时 100ms
    max_queue_size: 1000
    
  # 断路器
  circuit_breaker:
    enabled: true
    failure_threshold: 10   # 失败 10 次打开
    success_threshold: 5    # 成功 5 次关闭
    half_open_timeout: 30s
    
  # 监控告警
  monitoring:
    alert_redis_error: true
    alert_db_query: true
    alert_circuit_breaker_open: true
```

---

#### 监控和告警

```go
// 监控指标
type FallbackMetrics struct {
    RedisErrors int
    DBQueries   int
    DBErrors    int
    CacheHits   int
    CircuitBreakerOpen bool
}

// 告警规则
alerts:
  - name: RedisDown
    condition: redis_errors > 10 in 1m
    action: trigger_fallback_mode
    
  - name: DatabaseOverload
    condition: active_db_queries > max_concurrent
    action: reject_requests
    
  - name: CircuitBreakerOpen
    condition: circuit_breaker.state == "open"
    action: notify_ops_team
```

---

### 总结：降级策略的三层防护

```
┌─────────────────────────────────────┐
│  第1层：Redis（主要路径）           │ 快速 < 1ms
│  ├─ 正常情况下都用这个              │
│  └─ 故障率 < 0.1%                   │
└─────────────────────────────────────┘
           │ Redis 故障
           ▼
┌─────────────────────────────────────┐
│  第2层：本地缓存（降级方案1）        │ 极快 < 0.01ms
│  ├─ 启动时加载所有黑名单            │
│  ├─ 定期从 Redis/DB 同步             │
│  └─ 最多延迟 10 秒                   │
└─────────────────────────────────────┘
           │ 缓存未命中
           ▼
┌─────────────────────────────────────┐
│  第3层：限流保护的 DB（降级方案2）  │ 有限 < 100ms
│  ├─ 限制并发数 ≤ 20                 │
│  ├─ 单次查询超时 100ms              │
│  └─ 超限则拒绝请求                  │
└─────────────────────────────────────┘
           │ DB 无法处理
           ▼
┌─────────────────────────────────────┐
│  第4层：快速失败（最后防线）        │ 即时返回
│  ├─ 返回 503 Service Unavailable    │
│  └─ 告诉客户端系统故障              │
└─────────────────────────────────────┘
```

---

## 6. 封禁管理系统

### 6.1 封禁用户 API

```protobuf
// api/user/v1/ban.proto
message BanUserRequest {
    int64 user_id = 1;
    string ban_type = 2;      // "permanent" | "temporary"
    string reason = 3;
    int64 duration_hours = 4; // 临时封禁时长（小时）
}

message BanUserResponse {
    int32 code = 1;
    string message = 2;
    int64 ban_id = 3;
}
```

```go
// 实现逻辑
func (s *UserService) BanUser(ctx context.Context, req *BanUserRequest) error {
    var endTime *time.Time
    if req.BanType == "temporary" {
        t := time.Now().Add(time.Duration(req.DurationHours) * time.Hour)
        endTime = &t
    }
    
    // 创建封禁记录
    ban := &UserBan{
        UserID:    req.UserID,
        BanType:   req.BanType,
        Reason:    req.Reason,
        BannedBy:  getOperatorID(ctx),
        EndTime:   endTime,
        Status:    "active",
    }
    db.Create(ban)
    
    // 同步到 Redis（参见 3.2 节）
    syncBanToRedis(req.UserID)
    
    // 发送通知
    notifyUser(req.UserID, "Your account has been banned: " + req.Reason)
    
    return nil
}
```

### 6.2 解封用户 API

```protobuf
message UnbanUserRequest {
    int64 user_id = 1;
    string cancel_reason = 2;
}
```

### 6.3 查询封禁记录 API

```protobuf
message GetBanHistoryRequest {
    int64 user_id = 1;
    int32 page = 2;
    int32 page_size = 3;
}

message GetBanHistoryResponse {
    repeated BanRecord records = 1;
    int64 total = 2;
}

message BanRecord {
    int64 id = 1;
    string ban_type = 2;
    string reason = 3;
    int64 banned_by = 4;
    string banned_by_name = 5;
    string start_time = 6;
    string end_time = 7;
    string status = 8;
}
```

---

## 7. 定时任务 - 自动解封

### 7.1 临时封禁到期处理

```go
// 每分钟执行一次
func AutoUnbanCronJob() {
    // 查询所有应该解封的记录
    var bans []UserBan
    db.Where("status = 'active' AND ban_type = 'temporary' AND end_time <= NOW()").
        Find(&bans)
    
    for _, ban := range bans {
        // 更新封禁状态
        db.Model(&ban).Update("status", "expired")
        
        // 更新用户状态
        db.Model(&User{}).Where("id = ?", ban.UserID).Update("status", "active")
        
        // 从 Redis 移除
        redis.SRem("user:banned", ban.UserID)
        
        // 清除缓存
        redis.Del(fmt.Sprintf("user:info:%d", ban.UserID))
        
        // 发送通知
        notifyUser(ban.UserID, "Your ban has expired. Welcome back!")
        
        log.Infof("Auto unbanned user %d", ban.UserID)
    }
}
```

### 7.2 启动时同步封禁列表

```go
// 服务启动时执行
func InitBanListFromDB() {
    var bans []UserBan
    db.Where("status = 'active'").Select("user_id").Find(&bans)
    
    for _, ban := range bans {
        redis.SAdd("user:banned", ban.UserID)
    }
    
    log.Infof("Loaded %d banned users to Redis", len(bans))
}
```

---

## 8. 其他模块调用用户服务

### 8.1 场景一：判题服务提交代码

```go
// Judge Service 伪代码
func SubmitCode(ctx context.Context, req *SubmitRequest) {
    // 从 Gateway 注入的上下文获取 user_id（无需调用用户服务）
    userID := ctx.Value("user_id").(int64)
    
    // 直接使用 user_id 创建提交记录
    submission := &Submission{
        UserID:    userID,
        ProblemID: req.ProblemID,
        Code:      req.Code,
        Language:  req.Language,
    }
    db.Create(submission)
    
    // 推送到判题队列
    mq.Publish("judge.queue", submission)
}
```

### 8.2 场景二：比赛服务查询参赛者

```go
// Contest Service 伪代码
func GetContestRanking(contestID int64) []RankingItem {
    // 从数据库查询提交记录（已包含 user_id）
    submissions := db.Where("contest_id = ?", contestID).Find(&Submission{})
    
    // 批量获取用户信息（仅需要用户名等展示信息）
    userIDs := extractUserIDs(submissions)
    
    // 优先从 Redis 批量获取
    users := redis.MGet(userIDs.Map(func(id int64) string {
        return fmt.Sprintf("user:info:%d", id)
    }))
    
    // Redis 未命中的从数据库查询（或调用用户服务）
    missedIDs := findMissedUserIDs(users)
    if len(missedIDs) > 0 {
        // 可选：调用用户服务批量查询接口
        rpcUsers := userServiceClient.BatchGetUsers(missedIDs)
    }
    
    return buildRanking(submissions, users)
}
```

---

## 9. 安全性增强

### 9.1 密码修改流程

```go
func ChangePassword(userID int64, oldPassword, newPassword string) error {
    // 1. 验证旧密码
    user := db.First(&User{}, userID)
    if !bcrypt.Compare(user.PasswordHash, oldPassword) {
        return errors.New("old password incorrect")
    }
    
    // 2. 更新密码
    newHash := bcrypt.Hash(newPassword)
    db.Model(&user).Update("password_hash", newHash)
    
    // 3. 撤销所有现有 Token（强制重新登录）
    tokens := db.Where("user_id = ? AND revoked = 0", userID).Find(&UserToken{})
    for _, token := range tokens {
        redis.SAdd("token:blacklist", token.TokenHash)
        redis.Expire("token:blacklist", token.ExpiresAt.Sub(time.Now()))
    }
    db.Model(&UserToken{}).Where("user_id = ?", userID).Update("revoked", 1)
    
    // 4. 清除缓存
    redis.Del(fmt.Sprintf("user:info:%d", userID))
    
    return nil
}
```

### 9.2 登录限流防暴力破解

```go
func checkLoginRateLimit(identifier, ip string) error {
    // 用户名维度限流
    userKey := fmt.Sprintf("login:fail:%s", identifier)
    userFails := redis.Incr(userKey)
    redis.Expire(userKey, 15*time.Minute)
    
    if userFails > 5 {
        return errors.New("too many failed attempts for this account")
    }
    
    // IP 维度限流
    ipKey := fmt.Sprintf("login:fail:ip:%s", ip)
    ipFails := redis.Incr(ipKey)
    redis.Expire(ipKey, 15*time.Minute)
    
    if ipFails > 20 {
        return errors.New("too many failed attempts from this IP")
    }
    
    return nil
}
```

---

## 10. 监控与可观测性

### 10.1 关键指标

```yaml
# Prometheus Metrics
- jwt_validation_duration_seconds: JWT 验证耗时
- redis_banned_check_duration_seconds: Redis 封禁检查耗时
- auth_middleware_total: 鉴权中间件调用次数
- auth_middleware_failed_total: 鉴权失败次数（按原因分类）
- user_banned_total: 被封禁用户总数
- login_failed_total: 登录失败次数
```

### 10.2 告警规则

```yaml
# 封禁数量异常增长
- alert: BannedUserSurge
  expr: increase(user_banned_total[5m]) > 10
  annotations:
    summary: "大量用户被封禁"
    
# Redis 封禁检查失败
- alert: RedisCheckFailure
  expr: rate(redis_banned_check_errors[1m]) > 0.01
  annotations:
    summary: "Redis 封禁检查失败率过高"
```

---

## 11. REST API 设计

### 11.1 认证相关 API

#### 11.1.1 用户注册
```
POST /api/v1/user/register
Content-Type: application/json

Request:
{
    "username": "alice",
    "email": "alice@example.com",
    "password": "secure_password_123",
    "phone": "13800138000"  // 可选
}

Response (201 Created):
{
    "code": 0,
    "message": "success",
    "data": {
        "user_id": 10001,
        "username": "alice",
        "email": "alice@example.com",
        "created_at": "2024-02-01T10:00:00Z"
    }
}

Error Response (400):
{
    "code": 400,
    "message": "Username already exists"
}
```

#### 11.1.2 用户登录
```
POST /api/v1/user/login
Content-Type: application/json

Request:
{
    "identifier": "alice",          // username/email/phone
    "password": "secure_password_123",
    "device_info": "MacOS Chrome"   // 可选
}

Response (200 OK):
{
    "code": 0,
    "message": "success",
    "data": {
        "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
        "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
        "expires_in": 1800,              // 秒
        "user": {
            "user_id": 10001,
            "username": "alice",
            "email": "alice@example.com",
            "role": "user",
            "status": "active"
        }
    }
}

Error Response (401):
{
    "code": 401,
    "message": "Invalid username or password"
}
```

#### 11.1.3 刷新 Token
```
POST /api/v1/user/refresh-token
Content-Type: application/json

Request:
{
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}

Response (200 OK):
{
    "code": 0,
    "message": "success",
    "data": {
        "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
        "expires_in": 1800
    }
}

Error Response (401):
{
    "code": 401,
    "message": "Refresh token expired or invalid"
}
```

#### 11.1.4 用户退出登录
```
POST /api/v1/user/logout
Authorization: Bearer {access_token}
Content-Type: application/json

Request:
{
    "device_info": "MacOS Chrome"  // 可选，为空则登出所有设备
}

Response (200 OK):
{
    "code": 0,
    "message": "Logout successful"
}

Error Response (401):
{
    "code": 401,
    "message": "Unauthorized"
}
```

---

### 11.2 用户信息相关 API

#### 11.2.1 获取个人信息
```
GET /api/v1/user/profile
Authorization: Bearer {access_token}

Response (200 OK):
{
    "code": 0,
    "message": "success",
    "data": {
        "user_id": 10001,
        "username": "alice",
        "email": "alice@example.com",
        "phone": "13800138000",
        "role": "user",
        "status": "active",
        "created_at": "2024-02-01T10:00:00Z",
        "updated_at": "2024-02-01T10:00:00Z"
    }
}

Error Response (401):
{
    "code": 401,
    "message": "Unauthorized"
}
```

#### 11.2.2 修改密码
```
POST /api/v1/user/change-password
Authorization: Bearer {access_token}
Content-Type: application/json

Request:
{
    "old_password": "secure_password_123",
    "new_password": "new_secure_password_456"
}

Response (200 OK):
{
    "code": 0,
    "message": "Password changed successfully. Please login again."
}

Error Response (400):
{
    "code": 400,
    "message": "Old password is incorrect"
}
```

#### 11.2.3 更新个人信息
```
PUT /api/v1/user/profile
Authorization: Bearer {access_token}
Content-Type: application/json

Request:
{
    "email": "newemail@example.com",  // 可选
    "phone": "13900139000"             // 可选
}

Response (200 OK):
{
    "code": 0,
    "message": "success",
    "data": {
        "user_id": 10001,
        "username": "alice",
        "email": "newemail@example.com",
        "phone": "13900139000",
        "updated_at": "2024-02-01T11:00:00Z"
    }
}
```

---

### 11.3 封禁管理 API（仅管理员）

#### 11.3.1 封禁用户
```
POST /api/v1/admin/user/ban
Authorization: Bearer {admin_access_token}
Content-Type: application/json

Request:
{
    "user_id": 10001,
    "ban_type": "permanent",           // "permanent" 或 "temporary"
    "reason": "Cheating in contest",
    "duration_hours": 24               // 仅当 ban_type 为 "temporary" 时需要
}

Response (200 OK):
{
    "code": 0,
    "message": "User banned successfully",
    "data": {
        "ban_id": 1001,
        "user_id": 10001,
        "ban_type": "permanent",
        "reason": "Cheating in contest",
        "start_time": "2024-02-01T10:00:00Z",
        "end_time": null
    }
}

Error Response (403):
{
    "code": 403,
    "message": "Permission denied. Admin only."
}
```

#### 11.3.2 解封用户
```
POST /api/v1/admin/user/unban
Authorization: Bearer {admin_access_token}
Content-Type: application/json

Request:
{
    "user_id": 10001,
    "cancel_reason": "Appeal approved"
}

Response (200 OK):
{
    "code": 0,
    "message": "User unbanned successfully",
    "data": {
        "user_id": 10001,
        "status": "active"
    }
}
```

#### 11.3.3 查询用户封禁历史
```
GET /api/v1/admin/user/{user_id}/ban-history?page=1&page_size=10
Authorization: Bearer {admin_access_token}

Response (200 OK):
{
    "code": 0,
    "message": "success",
    "data": {
        "records": [
            {
                "id": 1001,
                "user_id": 10001,
                "ban_type": "permanent",
                "reason": "Cheating in contest",
                "banned_by": 1,
                "banned_by_name": "admin",
                "start_time": "2024-02-01T10:00:00Z",
                "end_time": null,
                "status": "active"
            }
        ],
        "total": 1,
        "page": 1,
        "page_size": 10
    }
}
```

#### 11.3.4 获取被封禁用户列表
```
GET /api/v1/admin/user/banned-list?page=1&page_size=20
Authorization: Bearer {admin_access_token}

Response (200 OK):
{
    "code": 0,
    "message": "success",
    "data": {
        "records": [
            {
                "user_id": 10001,
                "username": "alice",
                "email": "alice@example.com",
                "ban_type": "permanent",
                "reason": "Cheating in contest",
                "banned_at": "2024-02-01T10:00:00Z",
                "status": "active"
            },
            {
                "user_id": 10002,
                "username": "bob",
                "email": "bob@example.com",
                "ban_type": "temporary",
                "reason": "Spam comments",
                "banned_at": "2024-02-01T09:00:00Z",
                "unban_at": "2024-02-02T09:00:00Z",
                "status": "active"
            }
        ],
        "total": 2,
        "page": 1,
        "page_size": 20
    }
}
```

---

### 11.4 内部服务调用 API（服务间通信）

#### 11.4.1 获取单个用户信息
```
GET /api/v1/user/{user_id}
Authorization: Bearer {internal_service_token}

Response (200 OK):
{
    "code": 0,
    "message": "success",
    "data": {
        "user_id": 10001,
        "username": "alice",
        "email": "alice@example.com",
        "role": "user",
        "status": "active"
    }
}
```

#### 11.4.2 批量获取用户信息
```
POST /api/v1/user/batch-get
Authorization: Bearer {internal_service_token}
Content-Type: application/json

Request:
{
    "user_ids": [10001, 10002, 10003]
}

Response (200 OK):
{
    "code": 0,
    "message": "success",
    "data": {
        "users": [
            {
                "user_id": 10001,
                "username": "alice",
                "email": "alice@example.com",
                "role": "user",
                "status": "active"
            },
            {
                "user_id": 10002,
                "username": "bob",
                "email": "bob@example.com",
                "role": "problem_setter",
                "status": "active"
            }
        ]
    }
}
```

---

### 11.5 API 错误码定义

```
0       成功
400     请求参数错误
401     未授权（Token 无效或过期）
403     禁止访问（权限不足、用户被封禁）
404     资源不存在
409     冲突（用户名/邮箱已存在）
429     请求过于频繁（限流）
500     服务器内部错误
```

---

### 11.6 认证方式

所有需要认证的接口使用 Bearer Token：

```
Authorization: Bearer {access_token}
```

Token 包含以下信息（JWT Payload）：
```json
{
    "uid": 10001,
    "username": "alice",
    "role": "user",
    "exp": 1704096000,  // 过期时间
    "iat": 1704094200   // 签发时间
}
```

---

## 12. 完整的接口列表（gRPC）

### gRPC 服务定义

```protobuf
// api/user/v1/user_service.proto
syntax = "proto3";
package user.v1;

service UserService {
    // 用户认证
    rpc Register(RegisterRequest) returns (RegisterResponse);
    rpc Login(LoginRequest) returns (LoginResponse);
    rpc Logout(LogoutRequest) returns (LogoutResponse);
    rpc RefreshToken(RefreshTokenRequest) returns (RefreshTokenResponse);
    rpc ChangePassword(ChangePasswordRequest) returns (ChangePasswordResponse);
    
    // 封禁管理（仅管理员）
    rpc BanUser(BanUserRequest) returns (BanUserResponse);
    rpc UnbanUser(UnbanUserRequest) returns (UnbanUserResponse);
    rpc GetBanHistory(GetBanHistoryRequest) returns (GetBanHistoryResponse);
    rpc ListBannedUsers(ListBannedUsersRequest) returns (ListBannedUsersResponse);
    
    // 用户信息（内部调用）
    rpc GetUser(GetUserRequest) returns (GetUserResponse);
    rpc BatchGetUsers(BatchGetUsersRequest) returns (BatchGetUsersResponse);
    rpc UpdateUserProfile(UpdateUserProfileRequest) returns (UpdateUserProfileResponse);
}
```

---

## 13. 部署建议

### 13.1 服务配置

```yaml
# config.yaml
jwt:
  access_token_secret: "your-secret-key-change-in-production"
  refresh_token_secret: "your-refresh-secret-key"
  access_token_ttl: 30m
  refresh_token_ttl: 168h  # 7 days

redis:
  addr: "localhost:6379"
  db: 0
  pool_size: 100

mysql:
  dsn: "user:pass@tcp(127.0.0.1:3306)/fuzoj?charset=utf8mb4"
  max_open_conns: 100
  max_idle_conns: 10
```

### 13.2 性能优化建议

1. **Redis 连接池**：配置足够大的连接池以应对高并发
2. **MySQL 读写分离**：读操作走从库，降低主库压力
3. **JWT 密钥轮换**：定期更换签名密钥（需要通知所有 Gateway）
4. **批量查询优化**：使用 `MGET` 批量获取用户信息
5. **缓存预热**：服务启动时加载热点用户数据

---

## 14. 总结

### 核心优势

1. **高性能鉴权**：Gateway 层 JWT + Redis 验证，99% 请求不访问用户服务
2. **双层黑名单**：
   - Token 黑名单：短期撤销（退出/修改密码）
   - 用户封禁黑名单：长期封禁（基于 user_id）
3. **完整的审计追踪**：所有封禁操作可追溯
4. **自动化解封**：临时封禁到期自动处理
5. **四层高可用设计**：
   - 第1层：Redis（主要路径）< 1ms
   - 第2层：本地缓存（降级方案1）< 0.01ms
   - 第3层：限流保护的数据库（降级方案2）< 100ms
   - 第4层：快速失败（保护系统） 

### 性能指标（单机）

- JWT 验证：10万+ QPS
- Redis 黑名单检查：5万+ QPS
- 完整鉴权流程：3万+ QPS
- Gateway 鉴权延迟：< 2ms (P99)
