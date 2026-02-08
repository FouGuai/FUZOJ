# FuzOJ 用户模块设计（整理版，MySQL）

## 1. 目标与原则
- 高并发鉴权：请求不频繁调用用户服务。
- 封禁实时生效：封禁立刻阻断访问，同时可审计。
- 低频操作集中在 User Service：注册/登录/改密/封禁管理。

核心路径：Gateway 自验证 JWT + Redis 黑名单/封禁检查；业务服务只使用 `user_id`。

---

## 2. 模块职责划分
- Controller：参数校验、鉴权结果组装、错误码映射。
- Service：业务规则（登录、刷新、封禁、解封、改密、限流）。
- Repository：MySQL/Redis/MQ 读写与事务封装。

---

## 3. 数据模型（MySQL）

### 3.1 设计要点
- `users`：用户基础信息与状态。
- `user_bans`：封禁历史与审计数据。
- `user_tokens`：Token 记录（多设备、撤销）。

### 3.2 MySQL Schema（推荐）

```sql
-- 枚举类型（也可用 TEXT + CHECK 代替）
CREATE TYPE user_role AS ENUM (
  'guest', 'user', 'problem_setter', 'contest_manager', 'admin', 'super_admin'
);

CREATE TYPE user_status AS ENUM ('active', 'banned', 'pending_verify');

CREATE TYPE ban_type AS ENUM ('permanent', 'temporary');
CREATE TYPE ban_status AS ENUM ('active', 'expired', 'cancelled');

CREATE TYPE token_type AS ENUM ('access', 'refresh');

-- 用户表
CREATE TABLE users (
  id            BIGSERIAL PRIMARY KEY,
  username      VARCHAR(32)  NOT NULL,
  email         VARCHAR(128) NOT NULL,
  phone         VARCHAR(20),
  password_hash VARCRHA(128) NOT NULL,
  role          user_role    NOT NULL DEFAULT 'user',
  status        user_status  NOT NULL DEFAULT 'pending_verify',
  created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX users_username_uq ON users (username);
CREATE UNIQUE INDEX users_email_uq    ON users (email);
CREATE UNIQUE INDEX users_phone_uq    ON users (phone);
CREATE INDEX users_status_idx         ON users (status);

-- 封禁记录表
CREATE TABLE user_bans (
  id             BIGSERIAL PRIMARY KEY,
  user_id        BIGINT     NOT NULL REFERENCES users(id),
  ban_type       ban_type   NOT NULL DEFAULT 'permanent',
  reason         TEXT       NOT NULL,
  banned_by      BIGINT     NOT NULL, -- 管理员ID
  start_time     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  end_time       TIMESTAMPTZ,
  status         ban_status NOT NULL DEFAULT 'active',
  cancel_reason  TEXT,
  cancelled_by   BIGINT,
  cancelled_at   TIMESTAMPTZ,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX user_bans_user_id_idx  ON user_bans (user_id);
CREATE INDEX user_bans_status_idx   ON user_bans (status);
CREATE INDEX user_bans_end_time_idx ON user_bans (end_time);

-- Token 记录表
CREATE TABLE user_tokens (
  id         BIGSERIAL PRIMARY KEY,
  user_id    BIGINT      NOT NULL REFERENCES users(id),
  token_hash VARCHAR(64) NOT NULL,
  token_type token_type  NOT NULL DEFAULT 'access',
  device_info VARCHAR(256),
  ip_address  VARCHAR(45),
  expires_at  TIMESTAMPTZ NOT NULL,
  revoked     BOOLEAN     NOT NULL DEFAULT FALSE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX user_tokens_hash_uq ON user_tokens (token_hash);
CREATE INDEX user_tokens_user_id_idx   ON user_tokens (user_id);
CREATE INDEX user_tokens_expires_idx   ON user_tokens (expires_at);
```

说明：
- `password_hash` 使用 bcrypt（bcrypt 本身已包含盐，不建议额外 `salt` 字段）。
- 如果需要邮箱/用户名不区分大小写，建议使用 `citext` 或创建 `lower(email)`、`lower(username)` 唯一索引。

---

## 4. Redis 设计
```
user:info:{user_id}   -> Hash，TTL 30m（展示信息缓存）
user:banned           -> Set<user_id>（当前封禁）
token:blacklist       -> Set<token_hash>（撤销 token，TTL=token 过期）
login:fail:{id}       -> String 失败计数（TTL 15m）
```

---

## 5. 认证与鉴权流程

### 5.1 登录（思路）
1) 校验账号密码
2) 检查用户状态（banned/pending）
3) 生成 access/refresh token
4) 记录 token hash
5) 缓存用户展示信息

### 5.2 Gateway 鉴权（思路）
1) JWT 自验证
2) Redis token 黑名单检查
3) Redis 用户封禁检查
4) 注入 `user_id/role` 到上下文

---

## 6. 封禁与解封流程

### 6.1 封禁（思路）
1) MySQL 事务：写封禁记录 + 更新用户状态
2) 撤销所有 token：写入 `token:blacklist` + 标记 `revoked`
3) 发布封禁事件（Redis Pub/Sub + MQ）
4) 更新 `user:banned` 集合
5) 清理用户缓存

### 6.2 解封（思路）
1) MySQL 事务：更新封禁记录状态 + 恢复用户状态
2) 发布解封事件
3) 移除 `user:banned`
4) 清理用户缓存

---

## 7. 消息队列事件与消费者

### 7.1 事件结构（示意）
```
BanEvent:
- event_type: user.banned | user.unbanned
- user_id
- ban_type
- reason
- end_time
- operator_id
- timestamp
```

### 7.2 典型消费者
- Gateway：更新本地封禁缓存（关键）
- Judge：取消待判题提交
- Contest：移出比赛
- Audit：写审计日志
- Notification：用户通知（可选）

---

## 8. 高可用与降级策略
- 正常路径：Redis
- 降级路径：本地封禁缓存 -> 限流 DB 查询 -> 快速失败(503)
- 可选：Redis 断路器，连续失败触发降级
- 使用 BloomFilter 防止缓存击穿

---

## 9. 定时任务
- 自动解封：扫描 `user_bans` 到期记录，更新状态与缓存
- 启动同步：把 `active` 封禁加载到 Redis

---

## 10. API 设计（精简）

认证：
- `POST /api/v1/user/register`
- `POST /api/v1/user/login`
- `POST /api/v1/user/logout`
- `POST /api/v1/user/refresh-token`
- `POST /api/v1/user/change-password`

个人信息：
- `GET /api/v1/user/profile`
- `PUT /api/v1/user/profile`

管理员：
- `POST /api/v1/admin/user/ban`
- `POST /api/v1/admin/user/unban`
- `GET /api/v1/admin/user/{id}/ban-history`
- `GET /api/v1/admin/user/banned-list`

内部服务：
- `GET /api/v1/user/{id}`
- `POST /api/v1/user/batch-get`

---

## 11. 安全与风控
- 修改密码必须撤销全部 token
- 登录限流（账号维度 + IP 维度）
- 所有外部输入必须校验，内部接口必须鉴权

---

## 12. 监控指标（建议）
- JWT 验证耗时
- Redis 封禁检查耗时/失败率
- 鉴权失败次数（按原因分类）
- 封禁数量变化

---

## 13. 落地实现清单
- Model：User / UserBan / UserToken
- Repository：MySQL、Redis、MQ
- Service：AuthService / BanService
- Middleware：AuthMiddleware + Fallback
- Cron：AutoUnban

