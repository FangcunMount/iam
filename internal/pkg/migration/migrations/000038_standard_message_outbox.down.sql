-- Only an empty standard table can be removed. Published rows are retained
-- evidence too. Application rollback keeps schema 38 and unfinished ownership.
SET @iam_rm_rows = (SELECT COUNT(*) FROM rm_outbox);
SET @iam_rm_down_sql = IF(@iam_rm_rows = 0,
 'DROP TABLE rm_outbox',
 'SELECT iam_standard_outbox_rollback_requires_preserved_records()');
PREPARE iam_rm_down_stmt FROM @iam_rm_down_sql;
EXECUTE iam_rm_down_stmt;
DEALLOCATE PREPARE iam_rm_down_stmt;
