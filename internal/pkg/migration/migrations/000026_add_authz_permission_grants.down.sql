DROP TABLE IF EXISTS `authz_cutover_state`;
DROP TABLE IF EXISTS `authz_role_inheritances`;
DROP TABLE IF EXISTS `authz_permission_grants`;

SET @iam_has_attribute_schema = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'authz_resources'
      AND COLUMN_NAME = 'attribute_schema'
);
SET @iam_sql = IF(
    @iam_has_attribute_schema > 0,
    'ALTER TABLE `authz_resources` DROP COLUMN `attribute_schema`',
    'SELECT 1'
);
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;
