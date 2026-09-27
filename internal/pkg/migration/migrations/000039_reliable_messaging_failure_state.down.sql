-- Application rollback retains these additive columns and all standard rows.
-- Schema rollback is allowed only on a disposable, empty standard table.
SET @iam_rm_rows = (SELECT COUNT(*) FROM rm_outbox);
SET @iam_rm_down_sql = IF(@iam_rm_rows = 0,
 'ALTER TABLE rm_outbox DROP COLUMN failure_count, DROP COLUMN updated_at',
 'SELECT iam_standard_outbox_failure_state_rollback_requires_preserved_records()');
PREPARE iam_rm_down_stmt FROM @iam_rm_down_sql;
EXECUTE iam_rm_down_stmt;
DEALLOCATE PREPARE iam_rm_down_stmt;
