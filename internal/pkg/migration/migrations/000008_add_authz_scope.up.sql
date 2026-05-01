SET @iam_has_scope_kinds = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'authz_resources'
      AND COLUMN_NAME = 'scope_kinds'
);

SET @iam_sql = IF(@iam_has_scope_kinds = 0,
'ALTER TABLE `authz_resources`
     ADD COLUMN `scope_kinds` TEXT NULL COMMENT ''资源支持的授权范围类型 (JSON数组格式)'' AFTER `actions`',
'SELECT 1');
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

UPDATE `authz_resources`
SET `scope_kinds` = JSON_ARRAY('all')
WHERE `scope_kinds` IS NULL
   OR `scope_kinds` = '';

UPDATE `casbin_rule`
SET `v4` = 'all:*'
WHERE `ptype` = 'p'
  AND (`v4` IS NULL OR `v4` = '');
