CREATE TABLE IF NOT EXISTS rank_snapshot_meta (
  id BIGINT NOT NULL AUTO_INCREMENT,
  contest_id VARCHAR(64) NOT NULL,
  snapshot_at DATETIME(3) NOT NULL,
  last_result_id BIGINT NOT NULL DEFAULT 0,
  last_version BIGINT NOT NULL DEFAULT 0,
  total BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(16) NOT NULL DEFAULT 'writing',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY rank_snapshot_meta_contest_idx (contest_id, snapshot_at, status)
);

CREATE TABLE IF NOT EXISTS rank_snapshot_entry (
  snapshot_id BIGINT NOT NULL,
  member_id VARCHAR(64) NOT NULL,
  `rank` BIGINT NOT NULL,
  sort_score BIGINT NOT NULL,
  score_total BIGINT NOT NULL DEFAULT 0,
  penalty_total BIGINT NOT NULL DEFAULT 0,
  ac_count BIGINT NOT NULL DEFAULT 0,
  detail_json MEDIUMTEXT NULL,
  summary_json MEDIUMTEXT NOT NULL,
  PRIMARY KEY (snapshot_id, `rank`),
  KEY rank_snapshot_entry_member_idx (snapshot_id, member_id)
);
