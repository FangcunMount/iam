-- Migration: keep IAM identity anchors free of business-only attributes.

SET @iam_has_users_id_card_index = (
    SELECT COUNT(*)
    FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'users'
      AND INDEX_NAME = 'uk_id_card'
);

SET @iam_sql = IF(@iam_has_users_id_card_index > 0,
'ALTER TABLE `users` DROP INDEX `uk_id_card`',
'SELECT 1');
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

SET @iam_has_users_id_card = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'users'
      AND COLUMN_NAME = 'id_card'
);

SET @iam_sql = IF(@iam_has_users_id_card > 0,
'ALTER TABLE `users` DROP COLUMN `id_card`',
'SELECT 1');
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

SET @iam_has_profiles_height = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'profiles'
      AND COLUMN_NAME = 'height'
);

SET @iam_sql = IF(@iam_has_profiles_height > 0,
'ALTER TABLE `profiles` DROP COLUMN `height`',
'SELECT 1');
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

SET @iam_has_profiles_weight = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'profiles'
      AND COLUMN_NAME = 'weight'
);

SET @iam_sql = IF(@iam_has_profiles_weight > 0,
'ALTER TABLE `profiles` DROP COLUMN `weight`',
'SELECT 1');
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;
