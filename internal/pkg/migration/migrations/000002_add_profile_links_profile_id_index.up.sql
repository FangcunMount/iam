-- ============================================================================
-- Migration: Bridge legacy children/guardianships schema to profiles/profile_links
-- Version: 000002
-- Description:
--   * Deployed databases may already be at version 1 with physical tables
--     children/guardianships, because 000001 was edited after the UC refactor.
--   * This migration is the first version old databases actually execute, so
--     it must create and backfill the v2 tables before adding profile_id index.
--   * Legacy tables are intentionally kept for rollback/audit; runtime reads
--     profiles/profile_links only.
-- ============================================================================

CREATE TABLE IF NOT EXISTS `profiles`
(
    `id`         BIGINT UNSIGNED NOT NULL PRIMARY KEY COMMENT '档案ID',
    `name`       VARCHAR(64)     NOT NULL COMMENT '档案姓名',
    `id_card`    VARCHAR(20)              DEFAULT NULL COMMENT '身份证号码',
    `gender`     TINYINT         NOT NULL DEFAULT 0 COMMENT '性别: 0-未知, 1-男, 2-女',
    `birthday`   VARCHAR(10)              DEFAULT NULL COMMENT '出生日期 (YYYY-MM-DD)',
    `height`     BIGINT                   DEFAULT NULL COMMENT '身高 (以0.1cm为单位)',
    `weight`     BIGINT                   DEFAULT NULL COMMENT '体重 (以0.1kg为单位)',
    `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at` DATETIME                 DEFAULT NULL COMMENT '删除时间',
    `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创建人ID',
    `updated_by` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '更新人ID',
    `deleted_by` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除人ID',
    `version`    INT UNSIGNED    NOT NULL DEFAULT 1 COMMENT '乐观锁版本号',
    UNIQUE KEY `uk_id_card` (`id_card`),
    KEY `idx_deleted_at` (`deleted_at`),
    KEY `idx_name_gender_birthday` (`name`, `gender`, `birthday`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='档案表';

CREATE TABLE IF NOT EXISTS `profile_links`
(
    `id`             BIGINT UNSIGNED NOT NULL PRIMARY KEY COMMENT '档案关系ID',
    `user_id`        BIGINT UNSIGNED NOT NULL COMMENT '关系用户ID (用户ID)',
    `profile_id`     BIGINT UNSIGNED NOT NULL COMMENT '档案ID',
    `type`           VARCHAR(32)     NOT NULL DEFAULT 'relation' COMMENT '关系类型: self-本人档案, relation-普通关系',
    `relation`       VARCHAR(16)     NOT NULL COMMENT '档案关系: self-本人, parent-父母, grandparent-祖父母, other-其他',
    `self_key`       BIGINT UNSIGNED          DEFAULT NULL COMMENT 'active self link 唯一性保护键',
    `established_at` DATETIME        NOT NULL COMMENT '建立时间',
    `revoked_at`     DATETIME                 DEFAULT NULL COMMENT '撤销时间',
    `created_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`     DATETIME                 DEFAULT NULL COMMENT '删除时间',
    `created_by`     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创建人ID',
    `updated_by`     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '更新人ID',
    `deleted_by`     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除人ID',
    `version`        INT UNSIGNED    NOT NULL DEFAULT 1 COMMENT '乐观锁版本号',
    UNIQUE KEY `uk_user_profile_link` (`user_id`, `profile_id`, `type`),
    KEY `idx_type` (`type`),
    KEY `idx_deleted_at` (`deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='档案关系表';

SET @iam_has_children_table = (
    SELECT COUNT(*)
    FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'children'
);

SET @iam_sql = IF(@iam_has_children_table > 0,
'INSERT INTO `profiles` (`id`, `name`, `id_card`, `gender`, `birthday`, `height`, `weight`, `created_at`, `updated_at`,
                         `deleted_at`, `created_by`, `updated_by`, `deleted_by`, `version`)
 SELECT `id`, `name`, `id_card`, `gender`, `birthday`, `height`, `weight`, `created_at`, `updated_at`,
        `deleted_at`, `created_by`, `updated_by`, `deleted_by`, `version`
 FROM `children`
 ON DUPLICATE KEY UPDATE `name`       = VALUES(`name`),
                         `id_card`    = VALUES(`id_card`),
                         `gender`     = VALUES(`gender`),
                         `birthday`   = VALUES(`birthday`),
                         `height`     = VALUES(`height`),
                         `weight`     = VALUES(`weight`),
                         `deleted_at` = VALUES(`deleted_at`),
                         `created_by` = VALUES(`created_by`),
                         `updated_by` = VALUES(`updated_by`),
                         `deleted_by` = VALUES(`deleted_by`),
                         `version`    = VALUES(`version`),
                         `updated_at` = VALUES(`updated_at`)',
'SELECT 1');
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

SET @iam_has_guardianships_table = (
    SELECT COUNT(*)
    FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'guardianships'
);

SET @iam_sql = IF(@iam_has_guardianships_table > 0,
'INSERT INTO `profile_links` (`id`, `user_id`, `profile_id`, `type`, `relation`, `self_key`, `established_at`,
                              `revoked_at`, `created_at`, `updated_at`, `deleted_at`, `created_by`, `updated_by`,
                              `deleted_by`, `version`)
 SELECT `id`,
        `user_id`,
        `child_id`,
        CASE LOWER(TRIM(`relation`))
            WHEN ''self'' THEN ''self''
            ELSE ''relation''
        END,
        CASE LOWER(TRIM(`relation`))
            WHEN ''self'' THEN ''self''
            WHEN ''parent'' THEN ''parent''
            WHEN ''grandparent'' THEN ''grandparent''
            ELSE ''other''
        END,
        CASE
            WHEN LOWER(TRIM(`relation`)) = ''self'' AND `revoked_at` IS NULL AND `deleted_at` IS NULL THEN `user_id`
            ELSE NULL
        END,
        COALESCE(`established_at`, `created_at`, NOW()),
        `revoked_at`,
        COALESCE(`created_at`, NOW()),
        COALESCE(`updated_at`, `created_at`, NOW()),
        `deleted_at`,
        `created_by`,
        `updated_by`,
        `deleted_by`,
        `version`
 FROM `guardianships`
 ON DUPLICATE KEY UPDATE `user_id`        = VALUES(`user_id`),
                         `profile_id`     = VALUES(`profile_id`),
                         `type`           = VALUES(`type`),
                         `relation`       = VALUES(`relation`),
                         `self_key`       = VALUES(`self_key`),
                         `established_at` = VALUES(`established_at`),
                         `revoked_at`     = VALUES(`revoked_at`),
                         `deleted_at`     = VALUES(`deleted_at`),
                         `created_by`     = VALUES(`created_by`),
                         `updated_by`     = VALUES(`updated_by`),
                         `deleted_by`     = VALUES(`deleted_by`),
                         `version`        = VALUES(`version`),
                         `updated_at`     = VALUES(`updated_at`)',
'SELECT 1');
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

SET @iam_has_idx_profile_id = (
    SELECT COUNT(*)
    FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'profile_links'
      AND INDEX_NAME = 'idx_profile_id'
);

SET @iam_sql = IF(@iam_has_idx_profile_id = 0,
'CREATE INDEX `idx_profile_id` ON `profile_links` (`profile_id`)',
'SELECT 1');
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;
