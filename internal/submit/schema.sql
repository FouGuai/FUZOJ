-- Submit module schema for MySQL 8.0+

CREATE TABLE IF NOT EXISTS submissions (
  submission_id VARCHAR(64) PRIMARY KEY,
  problem_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  contest_id VARCHAR(64) NULL,
  language_id VARCHAR(32) NOT NULL,
  source_code MEDIUMTEXT NOT NULL,
  source_key VARCHAR(256) NOT NULL,
  source_hash CHAR(64) NOT NULL,
  scene VARCHAR(16) NOT NULL,
  final_status JSON NULL,
  final_status_at TIMESTAMP NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

  INDEX idx_submissions_user_created (user_id, created_at),
  INDEX idx_submissions_problem_created (problem_id, created_at),
  INDEX idx_submissions_contest_created (contest_id, created_at)
);
