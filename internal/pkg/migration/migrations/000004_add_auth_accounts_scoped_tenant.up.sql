SET @iam_has_scoped_tenant_id = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'auth_accounts'
      AND COLUMN_NAME = 'scoped_tenant_id'
);

SET @iam_sql = IF(@iam_has_scoped_tenant_id = 0,
'ALTER TABLE `auth_accounts`
     ADD COLUMN `scoped_tenant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT ''运营账号租户作用域，仅 type=opera 有效'' AFTER `external_id`',
'SELECT 1');
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

SET @iam_has_idx_scoped_tenant_id = (
    SELECT COUNT(*)
    FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'auth_accounts'
      AND INDEX_NAME = 'idx_scoped_tenant_id'
);

SET @iam_sql = IF(@iam_has_idx_scoped_tenant_id = 0,
'CREATE INDEX `idx_scoped_tenant_id` ON `auth_accounts` (`scoped_tenant_id`)',
'SELECT 1');
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;
