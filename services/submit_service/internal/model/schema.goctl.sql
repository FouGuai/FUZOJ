CREATE TABLE IF NOT EXISTS submissions (
  submission_id VARCHAR(64) NOT NULL,
  problem_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  contest_id VARCHAR(64) NULL,
  language_id VARCHAR(32) NOT NULL,
  source_code MEDIUMTEXT NOT NULL,
  source_key VARCHAR(512) NOT NULL,
  source_hash CHAR(64) NOT NULL,
  scene VARCHAR(32) NOT NULL DEFAULT 'practice',
  final_status JSON NULL,
  final_status_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (submission_id),
  KEY submissions_user_created_idx (user_id, created_at),
  KEY submissions_contest_created_idx (contest_id, created_at),
  KEY submissions_problem_created_idx (problem_id, created_at),
  KEY submissions_final_status_at_idx (final_status_at)
);

CREATE TABLE IF NOT EXISTS submission_logs (
  submission_id VARCHAR(64) NOT NULL,
  log_type VARCHAR(32) NOT NULL,
  test_id VARCHAR(64) NOT NULL DEFAULT '',
  content MEDIUMTEXT NULL,
  log_path VARCHAR(512) NULL,
  log_size BIGINT NOT NULL DEFAULT 0,
  truncated BOOLEAN NOT NULL DEFAULT FALSE,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (submission_id, log_type, test_id),
  KEY submission_logs_submission_id_idx (submission_id),
  KEY submission_logs_log_type_idx (log_type)
);
