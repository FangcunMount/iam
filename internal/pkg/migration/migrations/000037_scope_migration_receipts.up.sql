CREATE TABLE iam_scope_migrations (
 migration_id VARCHAR(64) NOT NULL PRIMARY KEY,
 actor_id VARCHAR(64) NOT NULL,
 fingerprint CHAR(64) NOT NULL,
 after_hash CHAR(64) NOT NULL,
 status VARCHAR(24) NOT NULL,
 rollback_actor_id VARCHAR(64) NOT NULL DEFAULT '',
 rollback_hash CHAR(64) NOT NULL DEFAULT '',
 rollback_json LONGTEXT NOT NULL,
 before_json LONGTEXT NOT NULL,
 after_json LONGTEXT NOT NULL,
 input_json LONGTEXT NOT NULL,
 plan_json LONGTEXT NOT NULL,
 created_at DATETIME(6) NOT NULL,
 updated_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
