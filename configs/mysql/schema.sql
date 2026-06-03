-- ============================================================================
-- IAM - Database Schema
-- Notes:
--   * All tables use `` prefix
--   * utf8mb4 / InnoDB
--   * Soft delete fields kept where applicable
--   * Added missing columns required by runtime baseline (e.g., is_system in roles)
--   * Cleaned malformed/merged blocks; removed duplicates
--   * Escaped reserved identifiers (e.g., `use`)
-- ============================================================================

-- Create database
CREATE DATABASE IF NOT EXISTS `iam`
    DEFAULT CHARACTER SET utf8mb4
    DEFAULT COLLATE utf8mb4_unicode_ci;

USE `iam`;

-- ============================================================================
-- Module 1: Identity 身份域
-- ============================================================================

-- 1.1 用户表
CREATE TABLE IF NOT EXISTS `users`
(
    `id`         BIGINT UNSIGNED NOT NULL PRIMARY KEY COMMENT '用户ID',
    `name`       VARCHAR(64)     NOT NULL COMMENT '用户名称',
    `nickname`   VARCHAR(64)              DEFAULT '' COMMENT '用户昵称',
    `phone`      VARCHAR(20)     NOT NULL COMMENT '手机号',
    `email`      VARCHAR(100)    NOT NULL COMMENT '邮箱',
    `id_card`    VARCHAR(20)     NOT NULL COMMENT '身份证号',
    `status`     INT             NOT NULL DEFAULT 1 COMMENT '用户状态: 1-正常, 2-禁用',
    `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at` DATETIME                 DEFAULT NULL COMMENT '删除时间',
    `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创建人ID',
    `updated_by` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '更新人ID',
    `deleted_by` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除人ID',
    `version`    INT UNSIGNED    NOT NULL DEFAULT 1 COMMENT '乐观锁版本号',
    UNIQUE KEY `uk_id_card` (`id_card`),
    KEY `idx_phone` (`phone`),
    KEY `idx_deleted_at` (`deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='用户表';

-- 1.2 档案表
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

-- 1.3 档案关系表
CREATE TABLE IF NOT EXISTS `profile_links`
(
    `id`             BIGINT UNSIGNED NOT NULL PRIMARY KEY COMMENT '档案关系ID',
    `user_id`        BIGINT UNSIGNED NOT NULL COMMENT '关系用户ID (用户ID)',
    `profile_id`       BIGINT UNSIGNED NOT NULL COMMENT '档案ID',
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
    UNIQUE KEY `uk_active_self_profile_link` (`self_key`),
    KEY `idx_type` (`type`),
    KEY `idx_deleted_at` (`deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='档案关系表';

-- ============================================================================
-- Module 2: Authentication (Authn)
-- ============================================================================

-- 2.1 登录身份绑定表（User -> LoginIdentity）
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

-- 2.2 认证凭据表（LoginIdentity -> Credential）
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

-- 2.3 Token 审计表（主存储在 Redis，此表仅用于审计）
CREATE TABLE IF NOT EXISTS `auth_token_audit`
(
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'ID',
    `token_id`       VARCHAR(64)     NOT NULL COMMENT 'Token ID (jti)',
    `token_type`     VARCHAR(16)     NOT NULL COMMENT 'Token类型: access|refresh',
    `user_id`        BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    `login_identity_id` BIGINT UNSIGNED NOT NULL COMMENT '登录身份ID',
    `tenant_id`      BIGINT UNSIGNED          DEFAULT NULL COMMENT '租户ID（可选）',
    `issued_at`      DATETIME        NOT NULL COMMENT '签发时间',
    `expires_at`     DATETIME        NOT NULL COMMENT '过期时间',
    `revoked_at`     DATETIME                 DEFAULT NULL COMMENT '撤销时间',
    `revoke_reason`  VARCHAR(64)              DEFAULT NULL COMMENT '撤销原因: logout|password_change|admin_revoke',
    `ip_address`     VARCHAR(45)              DEFAULT NULL COMMENT 'IP地址',
    `user_agent`     VARCHAR(500)             DEFAULT NULL COMMENT '浏览器UA',
    `device_id`      VARCHAR(100)             DEFAULT NULL COMMENT '设备ID',
    `created_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `created_by`     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创建人ID',
    `updated_by`     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '更新人ID',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_token_id` (`token_id`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_login_identity_id` (`login_identity_id`),
    KEY `idx_expires_at` (`expires_at`),
    KEY `idx_revoked_at` (`revoked_at`),
    KEY `idx_created_at` (`created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci
    COMMENT ='Token审计表 - 记录Token签发和撤销历史（主存储在Redis）';

-- 2.4 JWKS 密钥表
CREATE TABLE IF NOT EXISTS `jwks_keys`
(
    `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT '密钥ID',
    `kid`        VARCHAR(64)     NOT NULL COMMENT 'Key ID',
    `status`     TINYINT         NOT NULL DEFAULT 1 COMMENT '1-Active, 2-Grace, 3-Retired',
    `kty`        VARCHAR(32)     NOT NULL COMMENT 'Key Type: RSA/EC',
    `use`        VARCHAR(16)     NOT NULL COMMENT '密钥用途: sig/enc',
    `alg`        VARCHAR(32)     NOT NULL COMMENT '算法: RS256/RS384/RS512 等',
    `jwk_json`   JSON            NOT NULL COMMENT '公钥JWK JSON',
    `not_before` DATETIME                 DEFAULT NULL COMMENT '生效时间',
    `not_after`  DATETIME                 DEFAULT NULL COMMENT '过期时间',
    `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    UNIQUE KEY `uk_kid` (`kid`),
    KEY `idx_status` (`status`),
    KEY `idx_alg` (`alg`),
    KEY `idx_not_after` (`not_after`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='JWKS 密钥表';

-- 2.5 会话表
CREATE TABLE IF NOT EXISTS `auth_sessions`
(
    `id`              BIGINT UNSIGNED NOT NULL PRIMARY KEY COMMENT '会话ID (Snowflake)',
    `user_id`         BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    `refresh_token`   VARCHAR(255)    NOT NULL COMMENT 'Refresh Token',
    `access_token_id` VARCHAR(64)              DEFAULT NULL COMMENT '当前 Access Token ID (jti)',
    `device_id`       VARCHAR(100)             DEFAULT NULL COMMENT '设备ID',
    `device_type`     VARCHAR(32)              DEFAULT NULL COMMENT '设备类型 (ios/android/web/desktop)',
    `ip_address`      VARCHAR(45)              DEFAULT NULL COMMENT 'IP地址',
    `user_agent`      VARCHAR(500)             DEFAULT NULL COMMENT '浏览器UA',
    `expires_at`      TIMESTAMP       NOT NULL COMMENT '过期时间',
    `last_active_at`  TIMESTAMP       NULL     DEFAULT NULL COMMENT '最后活跃时间',
    `created_at`      TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`      TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    UNIQUE KEY `uk_refresh_token` (`refresh_token`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_expires_at` (`expires_at`),
    KEY `idx_device_id` (`device_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='会话表';

-- 2.6 已撤销访问令牌表
CREATE TABLE IF NOT EXISTS `auth_revoked_access_token`
(
    `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'ID',
    `token_id`   VARCHAR(64)     NOT NULL COMMENT 'Token ID (jti)',
    `user_id`    BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    `token_type` VARCHAR(20)     NOT NULL DEFAULT 'access' COMMENT 'Token类型 (access/refresh)',
    `reason`     VARCHAR(100)             DEFAULT NULL COMMENT '原因 (logout/password_change/admin_revoke)',
    `expires_at` TIMESTAMP       NOT NULL COMMENT '原过期时间',
    `created_at` TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_token_id` (`token_id`),
    KEY `idx_user_id` (`user_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='已撤销访问令牌表';

-- ============================================================================
-- Module 3: Authorization (Authz)
-- ============================================================================

-- 3.1 资源表
CREATE TABLE IF NOT EXISTS `authz_resources`
(
    `id`           BIGINT UNSIGNED NOT NULL PRIMARY KEY COMMENT '资源ID',
    `key`          VARCHAR(128)    NOT NULL COMMENT '资源唯一标识键',
    `display_name` VARCHAR(128)             DEFAULT NULL COMMENT '资源显示名称',
    `app_name`     VARCHAR(32)              DEFAULT NULL COMMENT '所属应用名称',
    `domain`       VARCHAR(32)              DEFAULT NULL COMMENT '资源域',
    `type`         VARCHAR(32)              DEFAULT NULL COMMENT '资源类型',
    `actions`      TEXT                     DEFAULT NULL COMMENT '资源可用操作 (JSON数组格式)',
    `scope_kinds`  TEXT                     DEFAULT NULL COMMENT '资源支持的授权范围类型 (JSON数组格式)',
    `description`  VARCHAR(512)             DEFAULT NULL COMMENT '资源描述',
    `created_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`   DATETIME                 DEFAULT NULL COMMENT '删除时间',
    `created_by`   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创建人ID',
    `updated_by`   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '更新人ID',
    `deleted_by`   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除人ID',
    `version`      INT UNSIGNED    NOT NULL DEFAULT 1 COMMENT '乐观锁版本号',
    UNIQUE KEY `uk_key` (`key`),
    KEY `idx_app_name` (`app_name`),
    KEY `idx_domain` (`domain`),
    KEY `idx_type` (`type`),
    KEY `idx_deleted_at` (`deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='资源表';

-- 3.2 角色表
CREATE TABLE IF NOT EXISTS `authz_roles`
(
    `id`           BIGINT UNSIGNED NOT NULL PRIMARY KEY COMMENT '角色ID',
    `name`         VARCHAR(64)     NOT NULL COMMENT '角色名称 (标识符)',
    `display_name` VARCHAR(128)             DEFAULT NULL COMMENT '角色显示名称',
    `tenant_id`    VARCHAR(64)     NOT NULL COMMENT '租户ID',
    `is_system`    TINYINT         NOT NULL DEFAULT 0 COMMENT '系统内置角色标识',
    `description`  VARCHAR(512)             DEFAULT NULL COMMENT '角色描述',
    `created_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`   DATETIME                 DEFAULT NULL COMMENT '删除时间',
    `created_by`   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创建人ID',
    `updated_by`   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '更新人ID',
    `deleted_by`   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除人ID',
    `version`      INT UNSIGNED    NOT NULL DEFAULT 1 COMMENT '乐观锁版本号',
    UNIQUE KEY `uk_tenant_name` (`tenant_id`, `name`),
    KEY `idx_tenant_id` (`tenant_id`),
    KEY `idx_deleted_at` (`deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='角色表';

-- 3.3 赋权表 (角色分配)
CREATE TABLE IF NOT EXISTS `authz_assignments`
(
    `id`           BIGINT UNSIGNED NOT NULL PRIMARY KEY COMMENT '赋权记录ID',
    `subject_type` VARCHAR(16)     NOT NULL COMMENT '主体类型: user/group',
    `subject_id`   VARCHAR(64)     NOT NULL COMMENT '主体ID',
    `role_id`      BIGINT UNSIGNED NOT NULL COMMENT '角色ID',
    `tenant_id`    VARCHAR(64)     NOT NULL COMMENT '租户ID',
    `granted_by`   VARCHAR(64)              DEFAULT NULL COMMENT '授权操作人',
    `granted_at`   DATETIME        NOT NULL COMMENT '授权时间',
    `created_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`   DATETIME                 DEFAULT NULL COMMENT '删除时间',
    `created_by`   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创建人ID',
    `updated_by`   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '更新人ID',
    `deleted_by`   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除人ID',
    `version`      INT UNSIGNED    NOT NULL DEFAULT 1 COMMENT '乐观锁版本号',
    KEY `idx_subject` (`subject_type`, `subject_id`),
    KEY `idx_role_id` (`role_id`),
    KEY `idx_tenant_id` (`tenant_id`),
    KEY `idx_deleted_at` (`deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='角色赋权表';

-- 3.4 策略版本表
CREATE TABLE IF NOT EXISTS `authz_policy_versions`
(
    `id`             BIGINT UNSIGNED NOT NULL PRIMARY KEY COMMENT '版本记录ID',
    `tenant_id`      VARCHAR(64)     NOT NULL COMMENT '租户ID',
    `policy_version` BIGINT          NOT NULL COMMENT '策略版本号',
    `changed_by`     VARCHAR(64)              DEFAULT NULL COMMENT '变更操作人',
    `reason`         VARCHAR(512)             DEFAULT NULL COMMENT '变更原因',
    `created_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`     DATETIME                 DEFAULT NULL COMMENT '删除时间',
    `created_by`     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创建人ID',
    `updated_by`     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '更新人ID',
    `deleted_by`     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除人ID',
    `version`        INT UNSIGNED    NOT NULL DEFAULT 1 COMMENT '乐观锁版本号',
    UNIQUE KEY `uk_tenant_version` (`tenant_id`, `policy_version`),
    KEY `idx_deleted_at` (`deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='策略版本表';

-- 3.5 Casbin 策略规则表
CREATE TABLE IF NOT EXISTS `casbin_rule`
(
    `id`    BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'ID',
    `ptype` VARCHAR(100)    NOT NULL COMMENT '策略类型 (p/g)',
    `v0`    VARCHAR(100) DEFAULT NULL COMMENT '值0 (subject)',
    `v1`    VARCHAR(100) DEFAULT NULL COMMENT '值1 (object)',
    `v2`    VARCHAR(100) DEFAULT NULL COMMENT '值2 (action)',
    `v3`    VARCHAR(100) DEFAULT NULL COMMENT '值3 (effect)',
    `v4`    VARCHAR(100) DEFAULT NULL COMMENT '值4 (扩展)',
    `v5`    VARCHAR(100) DEFAULT NULL COMMENT '值5 (扩展)',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_casbin_rule` (`ptype`, `v0`, `v1`, `v2`, `v3`, `v4`, `v5`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='Casbin 策略规则表 - 存储 RBAC 策略规则';

-- ============================================================================
-- Module 4: Identity Provider (IDP)
-- ============================================================================

-- 4.1 微信应用表
CREATE TABLE IF NOT EXISTS `idp_wechat_apps`
(
    `id`                     BIGINT UNSIGNED NOT NULL COMMENT '主键 ID (Snowflake)',
    `app_id`                 VARCHAR(64)     NOT NULL COMMENT '微信应用 ID (AppID)',
    `name`                   VARCHAR(255)    NOT NULL COMMENT '应用名称',
    `type`                   VARCHAR(32)     NOT NULL COMMENT '应用类型 (MiniProgram/MP/OpenPlatformWebsite)',
    `status`                 VARCHAR(32)     NOT NULL DEFAULT 'Enabled' COMMENT '应用状态 (Enabled/Disabled/Archived)',
    `auth_secret_cipher`     BLOB                     DEFAULT NULL COMMENT 'AppSecret 密文 (AES-GCM 加密)',
    `auth_secret_fp`         VARCHAR(128)             DEFAULT NULL COMMENT 'AppSecret 指纹 (SHA256)',
    `auth_secret_version`    INT             NOT NULL DEFAULT 0 COMMENT 'AppSecret 版本号',
    `auth_secret_rotated_at` DATETIME                 DEFAULT NULL COMMENT 'AppSecret 最后轮换时间',
    `msg_callback_token`     VARCHAR(128)             DEFAULT NULL COMMENT '消息推送回调 Token',
    `msg_aes_key_cipher`     BLOB                     DEFAULT NULL COMMENT '消息加密密钥密文 (EncodingAESKey)',
    `msg_secret_version`     INT             NOT NULL DEFAULT 0 COMMENT '消息密钥版本号',
    `msg_secret_rotated_at`  DATETIME                 DEFAULT NULL COMMENT '消息密钥最后轮换时间',
    `created_at`             DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`             DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_app_id` (`app_id`),
    KEY `idx_type` (`type`),
    KEY `idx_status` (`status`),
    KEY `idx_created_at` (`created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='微信应用表 - 管理微信小程序/公众号应用配置';


-- ============================================================================
-- Module 5: Platform / System
-- ============================================================================

-- 5.1 租户表
CREATE TABLE IF NOT EXISTS `tenants`
(
    `id`            VARCHAR(64)  NOT NULL COMMENT '租户ID',
    `name`          VARCHAR(100) NOT NULL COMMENT '租户名称',
    `code`          VARCHAR(50)  NOT NULL COMMENT '租户编码',
    `contact_name`  VARCHAR(100)          DEFAULT NULL COMMENT '联系人姓名',
    `contact_phone` VARCHAR(20)           DEFAULT NULL COMMENT '联系人电话',
    `contact_email` VARCHAR(100)          DEFAULT NULL COMMENT '联系人邮箱',
    `status`        VARCHAR(20)  NOT NULL DEFAULT 'active' COMMENT '状态 (active/inactive/suspended)',
    `max_users`     INT                   DEFAULT NULL COMMENT '最大用户数限制',
    `max_roles`     INT                   DEFAULT NULL COMMENT '最大角色数限制',
    `created_at`    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`    TIMESTAMP    NULL     DEFAULT NULL COMMENT '删除时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_code` (`code`),
    KEY `idx_status` (`status`),
    KEY `idx_deleted_at` (`deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='租户表 - 管理多租户信息';


-- 5.3 操作日志表
CREATE TABLE IF NOT EXISTS `operation_logs`
(
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'ID',
    `user_id`        BIGINT UNSIGNED          DEFAULT NULL COMMENT '操作用户ID',
    `tenant_id`      VARCHAR(64)              DEFAULT NULL COMMENT '租户ID',
    `operation_type` VARCHAR(50)     NOT NULL COMMENT '操作类型 (CREATE/UPDATE/DELETE/LOGIN/LOGOUT)',
    `resource_type`  VARCHAR(50)     NOT NULL COMMENT '资源类型 (user/role/resource/policy)',
    `resource_id`    VARCHAR(64)              DEFAULT NULL COMMENT '资源ID',
    `operation_desc` VARCHAR(500)             DEFAULT NULL COMMENT '操作描述',
    `ip_address`     VARCHAR(45)              DEFAULT NULL COMMENT 'IP地址',
    `user_agent`     VARCHAR(500)             DEFAULT NULL COMMENT '浏览器UA',
    `request_data`   TEXT                     DEFAULT NULL COMMENT '请求数据 (JSON)',
    `response_data`  TEXT                     DEFAULT NULL COMMENT '响应数据 (JSON)',
    `status`         VARCHAR(20)     NOT NULL COMMENT '状态 (success/failure)',
    `error_message`  VARCHAR(500)             DEFAULT NULL COMMENT '错误信息',
    `duration_ms`    INT                      DEFAULT NULL COMMENT '操作耗时 (毫秒)',
    `created_at`     TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (`id`),
    KEY `idx_iam_operation_logs_user_id` (`user_id`),
    KEY `idx_iam_operation_logs_tenant_id` (`tenant_id`),
    KEY `idx_iam_operation_logs_operation_type` (`operation_type`),
    KEY `idx_iam_operation_logs_resource_type` (`resource_type`),
    KEY `idx_iam_operation_logs_created_at` (`created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='操作日志表 - 记录所有重要操作';

-- 5.4 审计日志表
CREATE TABLE IF NOT EXISTS `audit_logs`
(
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'ID',
    `event_id`       VARCHAR(64)     NOT NULL COMMENT '事件ID (UUID)',
    `event_type`     VARCHAR(50)     NOT NULL COMMENT '事件类型 (authentication/authorization/data_access)',
    `event_category` VARCHAR(50)     NOT NULL COMMENT '事件分类 (security/compliance/system)',
    `severity`       VARCHAR(20)     NOT NULL COMMENT '严重级别 (info/warning/error/critical)',
    `user_id`        BIGINT UNSIGNED          DEFAULT NULL COMMENT '用户ID',
    `subject`        VARCHAR(128)             DEFAULT NULL COMMENT '主体 (用户名/服务名)',
    `action`         VARCHAR(100)    NOT NULL COMMENT '动作',
    `object`         VARCHAR(256)             DEFAULT NULL COMMENT '对象 (资源标识)',
    `result`         VARCHAR(20)     NOT NULL COMMENT '结果 (success/failure/denied)',
    `ip_address`     VARCHAR(45)              DEFAULT NULL COMMENT 'IP地址',
    `details`        JSON                     DEFAULT NULL COMMENT '详细信息 (JSON)',
    `created_at`     TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_event_id` (`event_id`),
    KEY `idx_event_type` (`event_type`),
    KEY `idx_severity` (`severity`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_created_at` (`created_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='审计日志表 - 用于安全合规审计';

-- 5.5 领域事件 Outbox 表
CREATE TABLE IF NOT EXISTS `domain_event_outbox`
(
    `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'ID',
    `event_id`        VARCHAR(64)     NOT NULL COMMENT '事件ID',
    `event_type`      VARCHAR(128)    NOT NULL COMMENT '事件类型',
    `aggregate_type`  VARCHAR(64)     NOT NULL COMMENT '聚合类型',
    `aggregate_id`    VARCHAR(64)     NOT NULL COMMENT '聚合ID',
    `topic_name`      VARCHAR(128)    NOT NULL COMMENT 'MQ Topic',
    `payload_json`    LONGTEXT        NOT NULL COMMENT '事件载荷 JSON',
    `status`          VARCHAR(32)     NOT NULL COMMENT 'pending/publishing/published/failed',
    `attempt_count`   INT UNSIGNED    NOT NULL DEFAULT 0 COMMENT '投递尝试次数',
    `next_attempt_at` DATETIME(3)     NOT NULL COMMENT '下次可投递时间',
    `last_error`      TEXT                     DEFAULT NULL COMMENT '最近失败原因',
    `created_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    `updated_at`      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    `published_at`    DATETIME(3)              DEFAULT NULL COMMENT '发布时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_event_id` (`event_id`),
    KEY `idx_status_next_attempt_at` (`status`, `next_attempt_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='领域事件 Outbox 表';

-- 5.6 数据字典表
CREATE TABLE IF NOT EXISTS `data_dictionary`
(
    `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'ID',
    `dict_type`  VARCHAR(50)     NOT NULL COMMENT '字典类型 (user_status/gender/relation_type)',
    `dict_code`  VARCHAR(50)     NOT NULL COMMENT '字典编码',
    `dict_value` VARCHAR(200)    NOT NULL COMMENT '字典值',
    `dict_label` VARCHAR(200)    NOT NULL COMMENT '字典标签',
    `sort_order` INT             NOT NULL DEFAULT 0 COMMENT '排序',
    `is_default` TINYINT         NOT NULL DEFAULT 0 COMMENT '是否默认 (0=否 1=是)',
    `status`     TINYINT         NOT NULL DEFAULT 1 COMMENT '状态 (1=启用 0=禁用)',
    `remark`     VARCHAR(500)             DEFAULT NULL COMMENT '备注',
    `created_at` TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_type_code` (`dict_type`, `dict_code`),
    KEY `idx_type` (`dict_type`),
    KEY `idx_status` (`status`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='数据字典表 - 管理系统枚举值';

-- ============================================================================
-- Schema 版本管理
-- ============================================================================
CREATE TABLE IF NOT EXISTS `schema_version`
(
    `id`          INT         NOT NULL AUTO_INCREMENT COMMENT 'ID',
    `version`     VARCHAR(20) NOT NULL COMMENT 'Schema版本号',
    `description` VARCHAR(500)         DEFAULT NULL COMMENT '版本说明',
    `applied_at`  TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '应用时间',
    PRIMARY KEY (`id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='Schema 版本管理表';

-- 记录 Schema 版本
INSERT INTO `schema_version` (`version`, `description`)
VALUES ('2.0', '2025-10-31 - 完整的模块化 Schema 定义 (修订版)')
ON DUPLICATE KEY UPDATE `description`=VALUES(`description`);
