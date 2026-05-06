-- Migration: add DB guard for one active self ProfileLink per User.
-- Historical duplicate active self links are normalized using the same policy
-- as the historical domain self-link guard: keep the earliest self link and convert
-- later active self links to parent relations.

UPDATE `profile_links` pl
    JOIN `profile_links` keeper
    ON keeper.`user_id` = pl.`user_id`
        AND keeper.`type` = 'self'
        AND keeper.`revoked_at` IS NULL
        AND keeper.`deleted_at` IS NULL
        AND (keeper.`established_at` < pl.`established_at`
            OR (keeper.`established_at` = pl.`established_at` AND keeper.`id` < pl.`id`))
SET pl.`type`     = 'relation',
    pl.`relation` = 'parent';

SET @iam_has_self_key = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'profile_links'
      AND COLUMN_NAME = 'self_key'
);

SET @iam_sql = IF(@iam_has_self_key = 0,
'ALTER TABLE `profile_links`
     ADD COLUMN `self_key` BIGINT UNSIGNED DEFAULT NULL COMMENT ''active self link 唯一性保护键'' AFTER `relation`',
'SELECT 1');
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

UPDATE `profile_links`
SET `self_key` = `user_id`
WHERE `type` = 'self'
  AND `revoked_at` IS NULL
  AND `deleted_at` IS NULL;

SET @iam_has_uk_active_self_profile_link = (
    SELECT COUNT(*)
    FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'profile_links'
      AND INDEX_NAME = 'uk_active_self_profile_link'
);

SET @iam_sql = IF(@iam_has_uk_active_self_profile_link = 0,
'CREATE UNIQUE INDEX `uk_active_self_profile_link` ON `profile_links` (`self_key`)',
'SELECT 1');
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;
