-- Problem module schema for goctl model generation.
-- Keep index names stable for error mapping in repository code.

CREATE TABLE IF NOT EXISTS problem (
  id BIGINT NOT NULL AUTO_INCREMENT,
  title VARCHAR(128) NOT NULL,
  status TINYINT NOT NULL DEFAULT 0 COMMENT '0=draft,1=published,2=archived',
  owner_id BIGINT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY problem_owner_id_idx (owner_id),
  KEY problem_status_idx (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Problems';

CREATE TABLE IF NOT EXISTS problem_version (
  id BIGINT NOT NULL AUTO_INCREMENT,
  problem_id BIGINT NOT NULL,
  version INT NOT NULL,
  state TINYINT NOT NULL DEFAULT 0 COMMENT '0=draft,1=published',
  config_json JSON NOT NULL,
  manifest_hash VARCHAR(128) NOT NULL,
  data_pack_key VARCHAR(256) NOT NULL,
  data_pack_hash VARCHAR(128) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY problem_version_uq (problem_id, version),
  KEY problem_version_latest_idx (problem_id, state, version),
  KEY problem_version_problem_id_idx (problem_id),
  KEY problem_version_state_idx (state)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Problem versions';

CREATE TABLE IF NOT EXISTS problem_manifest (
  id BIGINT NOT NULL AUTO_INCREMENT,
  problem_version_id BIGINT NOT NULL,
  manifest_json JSON NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY problem_manifest_version_uq (problem_version_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Problem manifests';

CREATE TABLE IF NOT EXISTS problem_data_pack (
  id BIGINT NOT NULL AUTO_INCREMENT,
  problem_version_id BIGINT NOT NULL,
  object_key VARCHAR(256) NOT NULL,
  size_bytes BIGINT NOT NULL DEFAULT 0,
  md5 VARCHAR(128) NOT NULL DEFAULT '',
  sha256 VARCHAR(128) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY problem_data_pack_version_idx (problem_version_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Problem data packs';

CREATE TABLE IF NOT EXISTS problem_version_seq (
  problem_id BIGINT NOT NULL,
  next_version INT NOT NULL,
  PRIMARY KEY (problem_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Per-problem version sequence';

CREATE TABLE IF NOT EXISTS problem_data_pack_upload (
  id BIGINT NOT NULL AUTO_INCREMENT,
  problem_id BIGINT NOT NULL,
  version INT NOT NULL,
  idempotency_key VARCHAR(64) NOT NULL,
  bucket VARCHAR(128) NOT NULL,
  object_key VARCHAR(256) NOT NULL,
  upload_id VARCHAR(256) NOT NULL DEFAULT '' COMMENT 'MinIO multipart upload id',
  expected_size_bytes BIGINT NOT NULL DEFAULT 0,
  expected_sha256 VARCHAR(128) NOT NULL DEFAULT '',
  content_type VARCHAR(128) NOT NULL DEFAULT 'application/octet-stream',
  state TINYINT NOT NULL DEFAULT 0 COMMENT '0=uploading,1=completed,2=aborted,3=expired',
  expires_at DATETIME(3) NOT NULL,
  created_by BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY pdu_problem_idem_uq (problem_id, idempotency_key),
  UNIQUE KEY pdu_problem_version_uq (problem_id, version),
  KEY pdu_state_expires_idx (state, expires_at),
  KEY pdu_problem_state_idx (problem_id, state)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Data pack upload sessions';
