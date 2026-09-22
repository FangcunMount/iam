-- Only an unused additive schema can be removed. Application rollback after
-- use keeps these columns/digests and requires an exclusive, audited handoff.
-- Run readiness checks BEFORE asking the migrator to step down; a refused down
-- migration follows the normal dirty-version recovery procedure, never force it.
SET @iam_rm_used = (
 SELECT COUNT(*) FROM domain_event_outbox
 WHERE rm_claim_token IS NOT NULL OR rm_claim_version <> 0 OR rm_claim_count <> 0
    OR rm_lease_until IS NOT NULL OR rm_fingerprint IS NOT NULL
    OR status NOT IN ('pending', 'failed', 'publishing', 'published')
);
SET @iam_rm_down_sql = IF(@iam_rm_used = 0,
 'ALTER TABLE domain_event_outbox DROP INDEX idx_outbox_status_rm_lease, DROP INDEX idx_outbox_status_updated, DROP COLUMN rm_claim_token, DROP COLUMN rm_claim_version, DROP COLUMN rm_claim_count, DROP COLUMN rm_lease_until, DROP COLUMN rm_fingerprint',
 'SELECT iam_reliable_messaging_rollback_requires_coordinated_restore()');
PREPARE iam_rm_down_stmt FROM @iam_rm_down_sql;
EXECUTE iam_rm_down_stmt;
DEALLOCATE PREPARE iam_rm_down_stmt;
