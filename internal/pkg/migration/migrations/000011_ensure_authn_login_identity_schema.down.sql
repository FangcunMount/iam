DROP TABLE IF EXISTS `auth_credentials`;
DROP TABLE IF EXISTS `auth_login_identities`;

SET @iam_has_legacy_credentials = (
    SELECT COUNT(*)
    FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'auth_credentials_legacy'
);

SET @iam_has_auth_credentials = (
    SELECT COUNT(*)
    FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'auth_credentials'
);

SET @iam_sql = IF(@iam_has_legacy_credentials > 0 AND @iam_has_auth_credentials = 0,
    'RENAME TABLE `auth_credentials_legacy` TO `auth_credentials`',
    'SELECT 1'
);
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;
