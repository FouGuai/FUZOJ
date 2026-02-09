-- User module schema for MySQL 8.0+
-- Notes:
-- - Use utf8mb4 for full Unicode support.
-- - Keep index names stable for error mapping in repository code.

CREATE TABLE IF NOT EXISTS users (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  username VARCHAR(32) NOT NULL,
  email VARCHAR(128) NOT NULL,
  phone VARCHAR(20) NULL,
  password_hash VARCHAR(128) NOT NULL,
  role ENUM('guest', 'user', 'problem_setter', 'contest_manager', 'admin', 'super_admin') NOT NULL DEFAULT 'user',
  status ENUM('active', 'banned', 'pending_verify') NOT NULL DEFAULT 'pending_verify',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY users_username_uq (username),
  UNIQUE KEY users_email_uq (email),
  UNIQUE KEY users_phone_uq (phone),
  KEY users_status_idx (status),
  KEY users_role_idx (role)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='User accounts';

CREATE TABLE IF NOT EXISTS user_bans (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT NOT NULL,
  ban_type ENUM('permanent', 'temporary') NOT NULL DEFAULT 'permanent',
  reason TEXT NOT NULL,
  banned_by BIGINT NOT NULL,
  start_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  end_time DATETIME(3) NULL,
  status ENUM('active', 'expired', 'cancelled') NOT NULL DEFAULT 'active',
  cancel_reason TEXT NULL,
  cancelled_by BIGINT NULL,
  cancelled_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY user_bans_user_id_idx (user_id),
  KEY user_bans_status_idx (status),
  KEY user_bans_end_time_idx (end_time),
  CONSTRAINT user_bans_user_fk FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='User ban history';

CREATE TABLE IF NOT EXISTS user_tokens (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT NOT NULL,
  token_hash VARCHAR(64) NOT NULL,
  token_type ENUM('access', 'refresh') NOT NULL DEFAULT 'access',
  device_info VARCHAR(256) NULL,
  ip_address VARCHAR(45) NULL,
  expires_at DATETIME(3) NOT NULL,
  revoked BOOLEAN NOT NULL DEFAULT FALSE,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY user_tokens_hash_uq (token_hash),
  KEY user_tokens_user_id_idx (user_id),
  KEY user_tokens_expires_idx (expires_at),
  KEY user_tokens_revoked_idx (revoked),
  CONSTRAINT user_tokens_user_fk FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='User tokens';
