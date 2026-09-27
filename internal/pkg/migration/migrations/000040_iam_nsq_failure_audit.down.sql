-- Retain every recorded terminal failure during application rollback.
SET @iam_failed_rows = (SELECT COUNT(*) FROM iam_nsq_failure_audit);
SET @iam_failed_down_sql = IF(@iam_failed_rows = 0,
 'DROP TABLE iam_nsq_failure_audit',
 'SELECT iam_nsq_failure_audit_rollback_requires_preserved_records()');
PREPARE iam_failed_down_stmt FROM @iam_failed_down_sql;
EXECUTE iam_failed_down_stmt;
DEALLOCATE PREPARE iam_failed_down_stmt;
