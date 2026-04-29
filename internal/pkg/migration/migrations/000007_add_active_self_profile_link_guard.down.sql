-- Migration Rollback: remove active self ProfileLink guard.

DROP INDEX `uk_active_self_profile_link` ON `profile_links`;

ALTER TABLE `profile_links`
    DROP COLUMN `self_key`;
