-- Migration: add DB guard for one active self ProfileLink per User.
-- Historical duplicate active self links are normalized using the same policy
-- as the domain SelfProfileEnsurer: keep the earliest self link and convert
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

ALTER TABLE `profile_links`
    ADD COLUMN `self_key` BIGINT UNSIGNED DEFAULT NULL COMMENT 'active self link 唯一性保护键' AFTER `relation`;

UPDATE `profile_links`
SET `self_key` = `user_id`
WHERE `type` = 'self'
  AND `revoked_at` IS NULL
  AND `deleted_at` IS NULL;

CREATE UNIQUE INDEX `uk_active_self_profile_link` ON `profile_links` (`self_key`);
