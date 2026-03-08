CREATE TABLE IF NOT EXISTS contests (
  contest_id VARCHAR(64) NOT NULL,
  title VARCHAR(255) NOT NULL,
  description TEXT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'draft',
  visibility VARCHAR(32) NOT NULL DEFAULT 'public',
  owner_id BIGINT NOT NULL DEFAULT 0,
  org_id BIGINT NOT NULL DEFAULT 0,
  start_at DATETIME(3) NOT NULL,
  end_at DATETIME(3) NOT NULL,
  rule_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (contest_id),
  KEY contests_status_idx (status),
  KEY contests_owner_idx (owner_id),
  KEY contests_org_idx (org_id),
  KEY contests_start_idx (start_at)
);

CREATE TABLE IF NOT EXISTS contest_problems (
  contest_id VARCHAR(64) NOT NULL,
  problem_id BIGINT NOT NULL,
  `order` INT NOT NULL DEFAULT 0,
  score INT NOT NULL DEFAULT 0,
  visible TINYINT(1) NOT NULL DEFAULT 1,
  version INT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (contest_id, problem_id),
  KEY contest_problems_contest_order_idx (contest_id, `order`)
);

CREATE TABLE IF NOT EXISTS contest_participants (
  contest_id VARCHAR(64) NOT NULL,
  user_id BIGINT NOT NULL,
  team_id VARCHAR(64) NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'registered',
  registered_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (contest_id, user_id),
  KEY contest_participants_contest_status_idx (contest_id, status),
  KEY contest_participants_team_idx (team_id)
);
