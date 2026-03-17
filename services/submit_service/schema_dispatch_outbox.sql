CREATE TABLE IF NOT EXISTS submission_dispatch_outbox (
  id BIGINT NOT NULL AUTO_INCREMENT,
  submission_id VARCHAR(64) NOT NULL,
  scene VARCHAR(32) NOT NULL DEFAULT '',
  contest_id VARCHAR(64) NULL,
  payload JSON NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  retry_count INT NOT NULL DEFAULT 0,
  next_retry_at DATETIME(3) NOT NULL,
  owner_id VARCHAR(128) NULL,
  lease_until DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY submission_dispatch_outbox_submission_uq (submission_id),
  KEY submission_dispatch_outbox_pending_idx (status, next_retry_at, id),
  KEY submission_dispatch_outbox_lease_idx (status, lease_until),
  KEY submission_dispatch_outbox_gc_idx (status, updated_at)
);
