-- IAM owns the terminal failure audit in its existing MySQL database. The
-- SDK never owns this schema or a second database connection.
CREATE TABLE iam_nsq_failure_audit (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  identity_hash BINARY(32) NOT NULL,
  topic VARCHAR(128) NOT NULL,
  channel_name VARCHAR(128) NOT NULL,
  application_id VARCHAR(255) NOT NULL,
  first_transport_id VARCHAR(64) NOT NULL,
  last_transport_id VARCHAR(64) NOT NULL,
  metadata_json JSON NOT NULL,
  metadata_hash BINARY(32) NOT NULL,
  payload LONGBLOB NOT NULL,
  payload_hash BINARY(32) NOT NULL,
  first_cause TEXT NOT NULL,
  last_cause TEXT NOT NULL,
  attempts INT UNSIGNED NOT NULL,
  source_timestamp_ns BIGINT NOT NULL,
  seen_count BIGINT UNSIGNED NOT NULL DEFAULT 1,
  first_seen_unix_ms BIGINT NOT NULL,
  last_seen_unix_ms BIGINT NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY iam_nsq_failure_identity (identity_hash),
  KEY iam_nsq_failure_recent (last_seen_unix_ms)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
