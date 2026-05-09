-- ============================================================================
-- IAM - System Bootstrap Data
-- Description: Idempotent baseline data migrated from the retired seeddata flow.
-- Scope:
--   - baseline tenants / users / opera accounts / password credentials
--   - IAM + QS roles / resources / assignments / Casbin policies
--   - default dictionaries and default WeChat app metadata
-- Non-scope:
--   - JWKS key material
--   - family/test/demo business data
--   - cross-service bootstrap side effects (QS / Collection / gRPC)
-- ============================================================================

USE `iam`;

-- ----------------------------------------------------------------------------
-- Tenants
-- ----------------------------------------------------------------------------
INSERT INTO `tenants` (`id`, `name`, `code`, `contact_name`, `contact_phone`, `contact_email`, `status`, `max_users`,
                       `max_roles`)
VALUES ('fangcun', '方寸', 'fangcun', 'admin', '15711236163', 'yshujie@163.com', 'active', 1000, 100),
       ('platform', '平台控制面', 'platform', 'system', '', 'system@fangcunmount.com', 'active', 100, 100)
ON DUPLICATE KEY UPDATE `name`          = VALUES(`name`),
                        `code`          = VALUES(`code`),
                        `contact_name`  = VALUES(`contact_name`),
                        `contact_phone` = VALUES(`contact_phone`),
                        `contact_email` = VALUES(`contact_email`),
                        `status`        = VALUES(`status`),
                        `max_users`     = VALUES(`max_users`),
                        `max_roles`     = VALUES(`max_roles`),
                        `updated_at`    = CURRENT_TIMESTAMP;

-- ----------------------------------------------------------------------------
-- System users
-- ----------------------------------------------------------------------------
INSERT INTO `users` (`id`, `name`, `nickname`, `phone`, `email`, `id_card`, `status`, `created_at`, `updated_at`,
                     `deleted_at`, `created_by`, `updated_by`, `deleted_by`, `version`)
VALUES (10001, '系统用户', '', NULL, 'system@fangcunmount.com', NULL, 1, NOW(), NOW(), NULL, 0, 0, 0, 1),
       (110001, '租户管理员', '', NULL, 'admin@fangcunmount.com', NULL, 1, NOW(), NOW(), NULL, 0, 0, 0, 1),
       (110002, '内容管理员', '', NULL, 'content_manager@fangcunmount.com', NULL, 1, NOW(), NOW(), NULL, 0, 0, 0, 1)
ON DUPLICATE KEY UPDATE `name`       = VALUES(`name`),
                        `nickname`   = VALUES(`nickname`),
                        `phone`      = VALUES(`phone`),
                        `email`      = VALUES(`email`),
                        `id_card`    = VALUES(`id_card`),
                        `status`     = VALUES(`status`),
                        `deleted_at` = NULL,
                        `deleted_by` = 0,
                        `updated_at` = NOW(),
                        `updated_by` = 0;

-- ----------------------------------------------------------------------------
-- Operation login identities
-- ----------------------------------------------------------------------------
INSERT INTO `auth_login_identities` (`id`, `user_id`, `provider`, `realm`, `identifier`, `global_identifier`,
                                     `status`, `verified_at`, `linked_at`, `profile_json`, `meta_json`, `created_at`,
                                     `updated_at`, `deleted_at`, `created_by`, `updated_by`, `deleted_by`, `version`)
VALUES (910100001, 10001, 'username', '1', 'system@fangcunmount.com', NULL, 'active', NOW(), NOW(), NULL, NULL, NOW(),
        NOW(), NULL, 0, 0, 0, 1),
       (910100002, 110001, 'username', '1', 'admin@fangcunmount.com', NULL, 'active', NOW(), NOW(), NULL, NULL, NOW(),
        NOW(), NULL, 0, 0, 0, 1),
       (910100003, 110002, 'username', '1', 'content_manager@fangcunmount.com', NULL, 'active', NOW(), NOW(), NULL,
        NULL, NOW(), NOW(), NULL, 0, 0, 0, 1)
ON DUPLICATE KEY UPDATE `user_id`           = VALUES(`user_id`),
                        `provider`          = VALUES(`provider`),
                        `realm`             = VALUES(`realm`),
                        `identifier`        = VALUES(`identifier`),
                        `global_identifier` = VALUES(`global_identifier`),
                        `status`            = VALUES(`status`),
                        `verified_at`       = VALUES(`verified_at`),
                        `profile_json`      = VALUES(`profile_json`),
                        `meta_json`         = VALUES(`meta_json`),
                        `deleted_at`        = NULL,
                        `deleted_by`        = 0,
                        `updated_at`        = NOW(),
                        `updated_by`        = 0;

-- 默认密码: Admin@123
INSERT INTO `auth_credentials` (`id`, `login_identity_id`, `type`, `material`, `algo`, `params_json`, `status`,
                                `failed_attempts`, `locked_until`, `last_success_at`,
                                `last_failure_at`, `created_at`, `updated_at`, `deleted_at`, `created_by`, `updated_by`,
                                `deleted_by`, `version`)
VALUES (910110001, 910100001, 'password',
        '$argon2id$v=19$m=65536,t=3,p=4$VnUrAyUWQMFItPHq5Tdyig$oWRC7CsasuR9vhAlYmE3GgqGM8RWsAE1jDwQuD9RRNg',
        'argon2id', NULL, 'enabled', 0, NULL, NULL, NULL, NOW(), NOW(), NULL, 0, 0, 0, 1),
       (910110002, 910100002, 'password',
        '$argon2id$v=19$m=65536,t=3,p=4$VnUrAyUWQMFItPHq5Tdyig$oWRC7CsasuR9vhAlYmE3GgqGM8RWsAE1jDwQuD9RRNg',
        'argon2id', NULL, 'enabled', 0, NULL, NULL, NULL, NOW(), NOW(), NULL, 0, 0, 0, 1),
       (910110003, 910100003, 'password',
        '$argon2id$v=19$m=65536,t=3,p=4$VnUrAyUWQMFItPHq5Tdyig$oWRC7CsasuR9vhAlYmE3GgqGM8RWsAE1jDwQuD9RRNg',
        'argon2id', NULL, 'enabled', 0, NULL, NULL, NULL, NOW(), NOW(), NULL, 0, 0, 0, 1)
ON DUPLICATE KEY UPDATE `login_identity_id` = VALUES(`login_identity_id`),
                        `type`              = VALUES(`type`),
                        `material`          = VALUES(`material`),
                        `algo`              = VALUES(`algo`),
                        `params_json`       = VALUES(`params_json`),
                        `status`            = VALUES(`status`),
                        `failed_attempts`   = VALUES(`failed_attempts`),
                        `locked_until`      = VALUES(`locked_until`),
                        `last_success_at`   = VALUES(`last_success_at`),
                        `last_failure_at`   = VALUES(`last_failure_at`),
                        `deleted_at`        = NULL,
                        `deleted_by`        = 0,
                        `updated_at`        = NOW(),
                        `updated_by`        = 0;

-- ----------------------------------------------------------------------------
-- Default WeChat app metadata
-- ----------------------------------------------------------------------------
INSERT INTO `idp_wechat_apps` (`id`, `app_id`, `name`, `type`, `status`, `auth_secret_cipher`, `auth_secret_fp`,
                               `auth_secret_version`, `auth_secret_rotated_at`, `msg_callback_token`,
                               `msg_aes_key_cipher`, `msg_secret_version`, `msg_secret_rotated_at`, `created_at`,
                               `updated_at`, `deleted_at`, `created_by`, `updated_by`, `deleted_by`, `version`)
VALUES (613485615102571054, 'wx72ade250b619a649', '问卷笔记本小程序', 'MiniProgram', 'Enabled', NULL, NULL, 0, NULL, NULL,
        NULL, 0, NULL, NOW(), NOW(), NULL, 0, 0, 0, 1)
ON DUPLICATE KEY UPDATE `name`       = VALUES(`name`),
                        `type`       = VALUES(`type`),
                        `status`     = VALUES(`status`),
                        `deleted_at` = NULL,
                        `deleted_by` = 0,
                        `updated_at` = NOW(),
                        `updated_by` = 0;

-- ----------------------------------------------------------------------------
-- Roles
-- ----------------------------------------------------------------------------
INSERT INTO `authz_roles` (`id`, `name`, `display_name`, `tenant_id`, `is_system`, `description`, `created_at`,
                           `updated_at`, `created_by`, `updated_by`, `deleted_by`, `version`)
VALUES (900000001, 'super_admin', '平台超级管理员', 'platform', 1, '平台控制面的根角色', NOW(), NOW(), 0, 0, 0, 1),
       (900000002, 'platform:admin', '平台管理员', 'platform', 1, '平台控制面的日常管理角色', NOW(), NOW(), 0, 0, 0, 1),
       (900000003, 'iam:admin', 'IAM 管理员', 'platform', 1, 'IAM 控制面的管理角色', NOW(), NOW(), 0, 0, 0, 1),
       (1, 'super_admin', '租户超级管理员', 'fangcun', 1, '方寸默认租户的超级管理员角色', NOW(), NOW(), 0, 0, 0, 1),
       (2, 'tenant_admin', '租户管理员', 'fangcun', 1, '管理本租户内的所有资源', NOW(), NOW(), 0, 0, 0, 1),
       (3, 'user', '普通用户', 'fangcun', 1, '普通用户权限', NOW(), NOW(), 0, 0, 0, 1),
       (900000101, 'qs:admin', 'QS管理员', '1', 1, 'QS服务所有资源的管理权限', NOW(), NOW(), 0, 0, 0, 1),
       (900000102, 'qs:content_manager', '内容管理员', '1', 1, '问卷和量表的管理权限', NOW(), NOW(), 0, 0, 0, 1),
       (900000103, 'qs:evaluator', '评估员', '1', 1, '测评相关只读权限', NOW(), NOW(), 0, 0, 0, 1),
       (900000104, 'qs:staff', '普通员工', '1', 1, '基本查看权限', NOW(), NOW(), 0, 0, 0, 1),
       (900000105, 'qs:evaluation_plan_manager', '测评计划管理员', '1', 1, '测评计划的管理权限', NOW(), NOW(), 0, 0, 0,
        1)
ON DUPLICATE KEY UPDATE `display_name` = VALUES(`display_name`),
                        `tenant_id`    = VALUES(`tenant_id`),
                        `is_system`    = VALUES(`is_system`),
                        `description`  = VALUES(`description`),
                        `deleted_at`   = NULL,
                        `deleted_by`   = 0,
                        `updated_at`   = NOW(),
                        `updated_by`   = 0;

-- ----------------------------------------------------------------------------
-- Resources
-- ----------------------------------------------------------------------------
INSERT INTO `authz_resources` (`id`, `key`, `display_name`, `app_name`, `domain`, `type`, `actions`, `description`,
                               `created_at`, `updated_at`, `created_by`, `updated_by`, `deleted_by`, `version`)
VALUES (901000001, 'iam:profile', '个人资料', 'iam', 'identity', 'instance', JSON_ARRAY('read', 'update'),
        '当前用户自服务资料读取与更新', NOW(), NOW(), 0, 0, 0, 1),
       (901000002, 'iam:users', '用户管理', 'iam', 'identity', 'collection',
        JSON_ARRAY('read', 'search', 'create', 'update', 'deactivate', 'block', 'link_external_identity'),
        '用户资料、状态和外部身份关联管理', NOW(), NOW(), 0, 0, 0, 1),
       (901000003, 'iam:profiles', '档案', 'iam', 'identity', 'collection',
        JSON_ARRAY('read', 'list', 'search', 'create', 'update'), '档案查询、注册与更新', NOW(), NOW(), 0, 0, 0, 1),
       (901000004, 'iam:profile-links', '档案关系', 'iam', 'identity', 'collection',
        JSON_ARRAY('read', 'list', 'grant', 'update_relation', 'revoke', 'bulk_revoke', 'import'),
        '档案关系授予、更新、撤销与导入', NOW(), NOW(), 0, 0, 0, 1),
       (901000005, 'iam:roles', '角色管理', 'iam', 'authz', 'collection',
        JSON_ARRAY('create', 'read', 'update', 'delete', 'list'), '角色目录管理', NOW(), NOW(), 0, 0, 0, 1),
       (901000006, 'iam:assignments', '角色分配', 'iam', 'authz', 'collection',
        JSON_ARRAY('grant', 'revoke', 'delete', 'read'), '主体与角色分配管理', NOW(), NOW(), 0, 0, 0, 1),
       (901000007, 'iam:policies', '策略管理', 'iam', 'authz', 'collection',
        JSON_ARRAY('read', 'write', 'delete'), 'Casbin 策略规则管理', NOW(), NOW(), 0, 0, 0, 1),
       (901000008, 'iam:resources', '资源目录', 'iam', 'authz', 'collection',
        JSON_ARRAY('create', 'read', 'update', 'delete', 'list', 'validate_action'), '资源目录定义和动作校验', NOW(),
        NOW(), 0, 0, 0, 1),
       (901000009, 'iam:check', '权限判定', 'iam', 'authz', 'action', JSON_ARRAY('check'), '单次 PDP 权限判定', NOW(),
        NOW(), 0, 0, 0, 1),
       (901000010, 'iam:accounts', '账号管理', 'iam', 'authn', 'collection',
        JSON_ARRAY('read', 'update', 'enable', 'disable', 'set_unionid'), '认证账号读取、资料更新与启停用', NOW(), NOW(),
        0, 0, 0, 1),
       (901000011, 'iam:jwks', 'JWKS 密钥管理', 'iam', 'authn', 'collection',
        JSON_ARRAY('create', 'read', 'list', 'retire', 'force_retire', 'enter_grace', 'cleanup', 'list_publishable'),
        'JWT 签名密钥与发布管理', NOW(), NOW(), 0, 0, 0, 1),
       (901000012, 'iam:wechat_apps', '微信应用管理', 'iam', 'idp', 'collection',
        JSON_ARRAY('create', 'read', 'rotate_auth_secret', 'rotate_msg_secret', 'refresh_access_token',
                   'get_access_token'),
        '微信应用与令牌管理', NOW(), NOW(), 0, 0, 0, 1),
       (901000013, 'qs:questionnaires', '问卷管理', 'qs', 'questionnaire', 'collection',
        JSON_ARRAY('create', 'read', 'list', 'update', 'delete', 'publish', 'unpublish', 'archive', 'statistics'),
        '问卷创建、维护、发布与统计', NOW(), NOW(), 0, 0, 0, 1),
       (901000014, 'qs:scales', '量表管理', 'qs', 'scale', 'collection',
        JSON_ARRAY('create', 'read', 'list', 'update', 'delete', 'publish', 'unpublish', 'archive'),
        '量表创建、维护与发布', NOW(), NOW(), 0, 0, 0, 1),
       (901000015, 'qs:answersheets', '答卷管理', 'qs', 'answersheet', 'collection',
        JSON_ARRAY('read', 'list', 'statistics', 'admin_submit'), '答卷查询、统计与管理员提交', NOW(), NOW(), 0, 0, 0,
        1),
       (901000016, 'qs:assessments', '测评执行', 'qs', 'evaluation', 'collection',
        JSON_ARRAY('read', 'list', 'retry', 'batch_evaluate', 'statistics'), '测评任务、结果重试与批量执行', NOW(), NOW(),
        0, 0, 0, 1),
       (901000017, 'qs:reports', '测评报告', 'qs', 'evaluation', 'collection', JSON_ARRAY('read', 'list'),
        '测评报告查询', NOW(), NOW(), 0, 0, 0, 1),
       (901000018, 'qs:testees', '受试者管理', 'qs', 'actor', 'collection',
        JSON_ARRAY('read', 'list', 'update', 'analyze', 'statistics'), '受试者资料、分析与统计', NOW(), NOW(), 0, 0, 0,
        1),
       (901000019, 'qs:staff', '员工管理', 'qs', 'actor', 'collection',
        JSON_ARRAY('create', 'read', 'list', 'delete'), '员工创建、查询与删除', NOW(), NOW(), 0, 0, 0, 1),
       (901000020, 'qs:evaluation_plans', '测评计划', 'qs', 'plan', 'collection',
        JSON_ARRAY('create', 'read', 'list', 'update', 'pause', 'resume', 'cancel', 'enroll', 'terminate',
                   'statistics'),
        '测评计划生命周期与统计', NOW(), NOW(), 0, 0, 0, 1),
       (901000021, 'qs:evaluation_plan_tasks', '测评计划任务', 'qs', 'plan_task', 'collection',
        JSON_ARRAY('schedule', 'read', 'list', 'open', 'complete', 'expire', 'cancel'),
        '测评计划任务调度与状态流转', NOW(), NOW(), 0, 0, 0, 1),
       (901000022, 'qs:system_statistics', '系统统计', 'qs', 'statistics', 'collection', JSON_ARRAY('read'),
        '后台系统统计查询', NOW(), NOW(), 0, 0, 0, 1),
       (901000023, 'qs:statistics_jobs', '统计作业', 'qs', 'statistics', 'collection',
        JSON_ARRAY('sync', 'validate'), '统计同步与一致性校验', NOW(), NOW(), 0, 0, 0, 1),
       (901000024, 'qs:codes', '邀请码申请', 'qs', 'code', 'collection', JSON_ARRAY('apply'), '邀请码申请', NOW(), NOW(),
        0, 0, 0, 1)
ON DUPLICATE KEY UPDATE `display_name` = VALUES(`display_name`),
                        `app_name`     = VALUES(`app_name`),
                        `domain`       = VALUES(`domain`),
                        `type`         = VALUES(`type`),
                        `actions`      = VALUES(`actions`),
                        `description`  = VALUES(`description`),
                        `deleted_at`   = NULL,
                        `deleted_by`   = 0,
                        `updated_at`   = NOW(),
                        `updated_by`   = 0;

-- ----------------------------------------------------------------------------
-- Role assignments
-- ----------------------------------------------------------------------------
INSERT INTO `authz_assignments` (`id`, `subject_type`, `subject_id`, `role_id`, `tenant_id`, `granted_by`, `granted_at`,
                                 `created_at`, `updated_at`, `deleted_at`, `created_by`, `updated_by`, `deleted_by`,
                                 `version`)
SELECT `seed`.*
FROM (SELECT 902000001 AS `id`,
             'user'    AS `subject_type`,
             '10001'   AS `subject_id`,
             900000001 AS `role_id`,
             'platform' AS `tenant_id`,
             'system'  AS `granted_by`,
             NOW()     AS `granted_at`,
             NOW()     AS `created_at`,
             NOW()     AS `updated_at`,
             NULL      AS `deleted_at`,
             0         AS `created_by`,
             0         AS `updated_by`,
             0         AS `deleted_by`,
             1         AS `version`
      UNION ALL
      SELECT 902000002, 'user', '10001', 2, 'fangcun', 'system', NOW(), NOW(), NOW(), NULL, 0, 0, 0, 1
      UNION ALL
      SELECT 902000003, 'user', '10001', 900000101, '1', 'system', NOW(), NOW(), NOW(), NULL, 0, 0, 0, 1
      UNION ALL
      SELECT 902000004, 'user', '110001', 2, 'fangcun', 'system', NOW(), NOW(), NOW(), NULL, 0, 0, 0, 1
      UNION ALL
      SELECT 902000005, 'user', '110001', 900000101, '1', 'system', NOW(), NOW(), NOW(), NULL, 0, 0, 0, 1
      UNION ALL
      SELECT 902000006, 'user', '110002', 900000102, '1', 'system', NOW(), NOW(), NOW(), NULL, 0, 0, 0, 1) AS `seed`
WHERE NOT EXISTS(SELECT 1
                 FROM `authz_assignments` `a`
                 WHERE `a`.`subject_type` = `seed`.`subject_type`
                   AND `a`.`subject_id` = `seed`.`subject_id`
                   AND `a`.`role_id` = `seed`.`role_id`
                   AND `a`.`tenant_id` = `seed`.`tenant_id`
                   AND `a`.`deleted_at` IS NULL);

-- ----------------------------------------------------------------------------
-- Casbin policy baseline
-- ----------------------------------------------------------------------------
INSERT INTO `casbin_rule` (`ptype`, `v0`, `v1`, `v2`, `v3`, `v4`, `v5`)
SELECT `seed`.*
FROM (SELECT 'p'                  AS `ptype`,
             'role:super_admin'   AS `v0`,
             'platform'           AS `v1`,
             '*'                  AS `v2`,
             '.*'                 AS `v3`,
             'all:*'                 AS `v4`,
             NULL                 AS `v5`
      UNION ALL
      SELECT 'p', 'role:tenant_admin', 'fangcun', 'iam:users',
             'read|search|create|update|deactivate|block|link_external_identity', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:tenant_admin', 'fangcun', 'iam:profiles', 'read|list|search|create|update', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:tenant_admin', 'fangcun', 'iam:profile-links',
             'read|list|grant|update_relation|revoke|bulk_revoke|import', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:tenant_admin', 'fangcun', 'iam:roles', 'create|read|update|delete|list', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:tenant_admin', 'fangcun', 'iam:assignments', 'grant|revoke|delete|read', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:tenant_admin', 'fangcun', 'iam:policies', 'read|write|delete', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:tenant_admin', 'fangcun', 'iam:resources',
             'create|read|update|delete|list|validate_action', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:tenant_admin', 'fangcun', 'iam:check', 'check', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:tenant_admin', 'fangcun', 'iam:accounts', 'read|update|enable|disable', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:user', 'fangcun', 'iam:profile', 'read', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:user', 'fangcun', 'iam:profile', 'update', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:qs:admin', '1', 'qs:*', '.*', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:qs:content_manager', '1', 'qs:questionnaires',
             'create|read|list|update|delete|publish|unpublish|archive|statistics', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:qs:content_manager', '1', 'qs:scales',
             'create|read|list|update|delete|publish|unpublish|archive', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:qs:evaluator', '1', 'qs:answersheets', 'read|list|statistics', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:qs:evaluator', '1', 'qs:assessments', 'read|list|retry|batch_evaluate|statistics', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:qs:evaluator', '1', 'qs:reports', 'read|list', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:qs:evaluator', '1', 'qs:testees', 'read|list|analyze|statistics', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:qs:staff', '1', 'qs:testees', 'read|list', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:qs:evaluation_plan_manager', '1', 'qs:evaluation_plans',
             'create|read|list|update|pause|resume|cancel|enroll|terminate|statistics', 'all:*', NULL
      UNION ALL
      SELECT 'p', 'role:qs:evaluation_plan_manager', '1', 'qs:evaluation_plan_tasks',
             'schedule|read|list|open|complete|expire|cancel', 'all:*', NULL
      UNION ALL
      SELECT 'g', 'role:tenant_admin', 'role:user', 'fangcun', NULL, NULL, NULL
      UNION ALL
      SELECT 'g', 'role:qs:admin', 'role:qs:content_manager', '1', NULL, NULL, NULL
      UNION ALL
      SELECT 'g', 'role:qs:admin', 'role:qs:evaluator', '1', NULL, NULL, NULL
      UNION ALL
      SELECT 'g', 'role:qs:admin', 'role:qs:evaluation_plan_manager', '1', NULL, NULL, NULL
      UNION ALL
      SELECT 'g', 'role:qs:evaluator', 'role:qs:staff', '1', NULL, NULL, NULL
      UNION ALL
      SELECT 'g', 'role:qs:evaluation_plan_manager', 'role:qs:staff', '1', NULL, NULL, NULL
      UNION ALL
      SELECT 'g', 'user:10001', 'role:super_admin', 'platform', NULL, NULL, NULL
      UNION ALL
      SELECT 'g', 'user:10001', 'role:tenant_admin', 'fangcun', NULL, NULL, NULL
      UNION ALL
      SELECT 'g', 'user:10001', 'role:qs:admin', '1', NULL, NULL, NULL
      UNION ALL
      SELECT 'g', 'user:110001', 'role:tenant_admin', 'fangcun', NULL, NULL, NULL
      UNION ALL
      SELECT 'g', 'user:110001', 'role:qs:admin', '1', NULL, NULL, NULL
      UNION ALL
      SELECT 'g', 'user:110002', 'role:qs:content_manager', '1', NULL, NULL, NULL) AS `seed`
WHERE NOT EXISTS(SELECT 1
                 FROM `casbin_rule` `r`
                 WHERE `r`.`ptype` = `seed`.`ptype`
                   AND ((`r`.`v0` = `seed`.`v0`) OR (`r`.`v0` IS NULL AND `seed`.`v0` IS NULL))
                   AND ((`r`.`v1` = `seed`.`v1`) OR (`r`.`v1` IS NULL AND `seed`.`v1` IS NULL))
                   AND ((`r`.`v2` = `seed`.`v2`) OR (`r`.`v2` IS NULL AND `seed`.`v2` IS NULL))
                   AND ((`r`.`v3` = `seed`.`v3`) OR (`r`.`v3` IS NULL AND `seed`.`v3` IS NULL))
                   AND ((`r`.`v4` = `seed`.`v4`) OR (`r`.`v4` IS NULL AND `seed`.`v4` IS NULL))
                   AND ((`r`.`v5` = `seed`.`v5`) OR (`r`.`v5` IS NULL AND `seed`.`v5` IS NULL)));

-- ----------------------------------------------------------------------------
-- Policy versions
-- ----------------------------------------------------------------------------
INSERT INTO `authz_policy_versions` (`id`, `tenant_id`, `policy_version`, `changed_by`, `reason`, `created_at`,
                                     `updated_at`, `deleted_at`, `created_by`, `updated_by`, `deleted_by`, `version`)
VALUES (903000001, 'platform', 1, 'bootstrap', 'bootstrap baseline', NOW(), NOW(), NULL, 0, 0, 0, 1),
       (903000002, 'fangcun', 1, 'bootstrap', 'bootstrap baseline', NOW(), NOW(), NULL, 0, 0, 0, 1),
       (903000003, '1', 1, 'bootstrap', 'bootstrap baseline', NOW(), NOW(), NULL, 0, 0, 0, 1)
ON DUPLICATE KEY UPDATE `changed_by` = VALUES(`changed_by`),
                        `reason`     = VALUES(`reason`),
                        `deleted_at` = NULL,
                        `deleted_by` = 0,
                        `updated_at` = NOW(),
                        `updated_by` = 0;

-- ----------------------------------------------------------------------------
-- Data dictionary
-- ----------------------------------------------------------------------------
INSERT INTO `data_dictionary` (`dict_type`, `dict_code`, `dict_value`, `dict_label`, `sort_order`, `is_default`)
VALUES ('gender', '0', '0', '未知', 1, 1),
       ('gender', '1', '1', '男', 2, 0),
       ('gender', '2', '2', '女', 3, 0)
ON DUPLICATE KEY UPDATE `dict_label` = VALUES(`dict_label`),
                        `sort_order` = VALUES(`sort_order`),
                        `is_default` = VALUES(`is_default`);

INSERT INTO `data_dictionary` (`dict_type`, `dict_code`, `dict_value`, `dict_label`, `sort_order`, `is_default`)
VALUES ('user_status', '1', '1', '正常', 1, 1),
       ('user_status', '2', '2', '禁用', 2, 0),
       ('user_status', '3', '3', '删除', 3, 0)
ON DUPLICATE KEY UPDATE `dict_label` = VALUES(`dict_label`),
                        `sort_order` = VALUES(`sort_order`),
                        `is_default` = VALUES(`is_default`);

INSERT INTO `data_dictionary` (`dict_type`, `dict_code`, `dict_value`, `dict_label`, `sort_order`)
VALUES ('relation_type', 'self', 'self', '本人', 1),
       ('relation_type', 'father', 'father', '父亲', 2),
       ('relation_type', 'mother', 'mother', '母亲', 3),
       ('relation_type', 'grandfather', 'grandfather', '祖父/外祖父', 4),
       ('relation_type', 'grandmother', 'grandmother', '祖母/外祖母', 5),
       ('relation_type', 'other', 'other', '关系用户', 6)
ON DUPLICATE KEY UPDATE `dict_label` = VALUES(`dict_label`),
                        `sort_order` = VALUES(`sort_order`);
