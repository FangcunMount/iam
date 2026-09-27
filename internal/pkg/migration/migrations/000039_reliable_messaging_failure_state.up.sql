-- Additive SDK v0.2.1 fields. Old v0.1.0 writers can still append and claim.
-- Existing attempt_count cannot be backfilled into failure_count: lease claims
-- increase attempts without proving a publish failure.
ALTER TABLE rm_outbox
  ADD COLUMN failure_count BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER attempt_count,
  ADD COLUMN updated_at DATETIME(6) NOT NULL DEFAULT (UTC_TIMESTAMP(6)) AFTER created_at;
