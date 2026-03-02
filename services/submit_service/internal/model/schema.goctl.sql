CREATE TABLE submission_logs (
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
