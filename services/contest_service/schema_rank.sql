CREATE TABLE IF NOT EXISTS contest_member_problem_state (
  contest_id VARCHAR(64) NOT NULL,
  member_id VARCHAR(64) NOT NULL,
  problem_id BIGINT NOT NULL,
  solved TINYINT NOT NULL DEFAULT 0,
  first_ac_at DATETIME(3) NULL,
  wrong_count INT NOT NULL DEFAULT 0,
  score INT NOT NULL DEFAULT 0,
  penalty BIGINT NOT NULL DEFAULT 0,
  last_submission_id VARCHAR(64) NULL,
  last_submission_at DATETIME(3) NULL,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (contest_id, member_id, problem_id),
  KEY contest_member_idx (contest_id, member_id),
  KEY contest_problem_idx (contest_id, problem_id)
);

CREATE TABLE IF NOT EXISTS contest_member_summary_snapshot (
  contest_id VARCHAR(64) NOT NULL,
  member_id VARCHAR(64) NOT NULL,
  score_total BIGINT NOT NULL DEFAULT 0,
  penalty_total BIGINT NOT NULL DEFAULT 0,
  ac_count BIGINT NOT NULL DEFAULT 0,
  detail_json LONGTEXT NULL,
  version BIGINT NOT NULL DEFAULT 0,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (contest_id, member_id),
  KEY contest_member_summary_idx (contest_id, member_id)
);

CREATE TABLE IF NOT EXISTS contest_rank_outbox (
  id BIGINT NOT NULL AUTO_INCREMENT,
  contest_id VARCHAR(64) NOT NULL,
  event_key VARCHAR(128) NOT NULL,
  payload MEDIUMTEXT NOT NULL,
  status TINYINT NOT NULL DEFAULT 0,
  retry_count INT NOT NULL DEFAULT 0,
  next_retry_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  owner_id VARCHAR(64) NULL,
  lease_until DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY contest_rank_outbox_event_key_uq (event_key),
  KEY contest_rank_outbox_dispatch_idx (status, contest_id, next_retry_at, id),
  KEY contest_rank_outbox_pending_idx (status, next_retry_at, contest_id, id),
  KEY contest_rank_outbox_lease_idx (status, lease_until, id),
  KEY contest_rank_outbox_gc_idx (status, updated_at, id)
);

CREATE TABLE IF NOT EXISTS contest_rank_outbox_lock (
  contest_id VARCHAR(64) NOT NULL,
  owner_id VARCHAR(64) NOT NULL,
  lease_until DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (contest_id),
  KEY contest_rank_outbox_lock_lease_idx (lease_until)
);

CREATE TABLE IF NOT EXISTS contest_rank_result_seq (
  contest_id VARCHAR(64) NOT NULL,
  next_result_id BIGINT NOT NULL DEFAULT 1,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (contest_id)
);
