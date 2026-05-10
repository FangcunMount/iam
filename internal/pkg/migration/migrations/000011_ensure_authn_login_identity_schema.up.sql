-- Ensure the AuthN LoginIdentity/Credential schema exists on databases that
-- already ran older Account/Credential migrations.

-- Keep older auth_accounts readable for the one-off backfill. Some deployed
-- databases may not have received the old scoped_tenant_id migration.
SET @iam_has_auth_accounts = (
    SELECT COUNT(*)
    FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'auth_accounts'
);

SET @iam_has_account_scoped_tenant_id = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'auth_accounts'
      AND COLUMN_NAME = 'scoped_tenant_id'
);

SET @iam_sql = IF(@iam_has_auth_accounts > 0 AND @iam_has_account_scoped_tenant_id = 0,
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

SET @iam_sql = IF(@iam_has_auth_accounts > 0 AND @iam_has_idx_scoped_tenant_id = 0,
'CREATE INDEX `idx_scoped_tenant_id` ON `auth_accounts` (`scoped_tenant_id`)',
'SELECT 1');
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

-- auth_credentials used to be the legacy Account-bound credential table. The
-- new runtime needs auth_credentials to be LoginIdentity-bound, so preserve the
-- old table under a legacy name before creating the new structure.
SET @iam_has_auth_credentials = (
    SELECT COUNT(*)
    FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'auth_credentials'
);

SET @iam_has_legacy_credentials = (
    SELECT COUNT(*)
    FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'auth_credentials_legacy'
);

SET @iam_auth_credentials_has_account_id = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'auth_credentials'
      AND COLUMN_NAME = 'account_id'
);

SET @iam_auth_credentials_has_login_identity_id = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'auth_credentials'
      AND COLUMN_NAME = 'login_identity_id'
);

SET @iam_sql = IF(
    @iam_has_auth_credentials > 0
        AND @iam_auth_credentials_has_account_id > 0
        AND @iam_auth_credentials_has_login_identity_id = 0
        AND @iam_has_legacy_credentials = 0,
    'RENAME TABLE `auth_credentials` TO `auth_credentials_legacy`',
    'SELECT 1'
);
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

CREATE TABLE IF NOT EXISTS `auth_login_identities`
(
    `id`                BIGINT UNSIGNED NOT NULL COMMENT '登录身份ID（Snowflake）',
    `user_id`           BIGINT UNSIGNED NOT NULL COMMENT '关联 IAM User ID',
    `provider`          VARCHAR(32)     NOT NULL COMMENT '登录身份提供方: username|phone|wechat_minip|wecom',
    `realm`             VARCHAR(128)    NOT NULL DEFAULT '' COMMENT '身份命名空间: tenant_id|global|appid|corp_id',
    `identifier`        VARCHAR(255)    NOT NULL COMMENT '命名空间内标识: username|+E164|openid|userid',
    `global_identifier` VARCHAR(255)             DEFAULT NULL COMMENT '全局外部标识，如 unionid',
    `status`            VARCHAR(32)     NOT NULL COMMENT '状态: active|disabled|archived|deleted',
    `verified_at`       DATETIME                 DEFAULT NULL COMMENT '身份验证时间',
    `linked_at`         DATETIME        NOT NULL COMMENT '绑定时间',
    `profile_json`      JSON                     DEFAULT NULL COMMENT '身份资料快照',
    `meta_json`         JSON                     DEFAULT NULL COMMENT '额外元数据',
    `created_at`        DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`        DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`        DATETIME                 DEFAULT NULL COMMENT '删除时间（软删除）',
    `created_by`        BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创建人ID',
    `updated_by`        BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '更新人ID',
    `deleted_by`        BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除人ID',
    `version`           INT UNSIGNED    NOT NULL DEFAULT 1 COMMENT '乐观锁版本号',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_provider_realm_identifier` (`provider`, `realm`, `identifier`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_global_identifier` (`global_identifier`),
    KEY `idx_deleted_at` (`deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci
    COMMENT ='登录身份绑定表';

CREATE TABLE IF NOT EXISTS `auth_credentials`
(
    `id`                BIGINT UNSIGNED NOT NULL COMMENT '凭据ID（Snowflake）',
    `login_identity_id` BIGINT UNSIGNED NOT NULL COMMENT '关联登录身份ID',
    `type`              VARCHAR(32)     NOT NULL COMMENT '凭据类型: password|passkey|totp|recovery_code',
    `material`          VARBINARY(4096)          DEFAULT NULL COMMENT '认证材料: password hash|public key|encrypted secret',
    `algo`              VARCHAR(64)              DEFAULT NULL COMMENT '算法: argon2id|bcrypt|es256 等',
    `params_json`       JSON                     DEFAULT NULL COMMENT '参数或元数据',
    `status`            VARCHAR(32)     NOT NULL COMMENT '状态: enabled|disabled',
    `failed_attempts`   INT             NOT NULL DEFAULT 0 COMMENT '失败尝试次数',
    `locked_until`      DATETIME                 DEFAULT NULL COMMENT '锁定截止时间',
    `last_success_at`   DATETIME                 DEFAULT NULL COMMENT '最近成功时间',
    `last_failure_at`   DATETIME                 DEFAULT NULL COMMENT '最近失败时间',
    `created_at`        DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`        DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`        DATETIME                 DEFAULT NULL COMMENT '删除时间（软删除）',
    `created_by`        BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创建人ID',
    `updated_by`        BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '更新人ID',
    `deleted_by`        BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除人ID',
    `version`           INT UNSIGNED    NOT NULL DEFAULT 1 COMMENT '乐观锁版本号',
    PRIMARY KEY (`id`),
    KEY `idx_login_identity_id` (`login_identity_id`),
    UNIQUE KEY `uk_identity_type` (`login_identity_id`, `type`),
    KEY `idx_deleted_at` (`deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci
    COMMENT ='认证凭据表 - 仅保存 IAM 需要校验的长期认证材料';
