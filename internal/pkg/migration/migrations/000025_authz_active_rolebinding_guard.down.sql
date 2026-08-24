SET @iam_has_active_index = (
    SELECT COUNT(*)
    FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'authz_assignments'
      AND INDEX_NAME = 'uk_authz_assignments_active'
);
SET @iam_sql = IF(
    @iam_has_active_index > 0,
    'DROP INDEX `uk_authz_assignments_active` ON `authz_assignments`',
    'SELECT 1'
);
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

SET @iam_has_active_guard = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'authz_assignments'
      AND COLUMN_NAME = 'active_guard'
);
SET @iam_sql = IF(
    @iam_has_active_guard > 0,
    'ALTER TABLE `authz_assignments` DROP COLUMN `active_guard`',
    'SELECT 1'
);
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;
