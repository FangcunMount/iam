-- ============================================================================
-- IAM - System Bootstrap Data
-- Description: Idempotent baseline data migrated from the retired seeddata flow.
-- Scope:
--   - baseline users / operation login identities / password credentials
--   - IAM + QS roles / resources / assignments / Casbin policies
--   - default WeChat app metadata
-- Non-scope:
--   - JWKS key material
--   - family/test/demo business data
--   - cross-service bootstrap side effects (QS / Collection / gRPC)
-- ============================================================================

-- ----------------------------------------------------------------------------
-- System users
-- ----------------------------------------------------------------------------
INSERT INTO `users` (`id`, `name`, `nickname`, `phone`, `email`, `status`, `created_at`, `updated_at`,
                     `deleted_at`, `created_by`, `updated_by`, `deleted_by`, `version`)
VALUES (10001, '系统用户', '', NULL, 'system@fangcunmount.com', 1, NOW(), NOW(), NULL, 0, 0, 0, 1),
       (110001, '租户管理员', '', NULL, 'admin@fangcunmount.com', 1, NOW(), NOW(), NULL, 0, 0, 0, 1),
       (110002, '内容管理员', '', NULL, 'content_manager@fangcunmount.com', 1, NOW(), NOW(), NULL, 0, 0, 0, 1)
ON DUPLICATE KEY UPDATE `name`       = VALUES(`name`),
                        `nickname`   = VALUES(`nickname`),
                        `phone`      = VALUES(`phone`),
                        `email`      = VALUES(`email`),
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
       (900000101, 'qs:admin', 'QS管理员', 'fangcun', 1, 'QS服务所有资源的管理权限', NOW(), NOW(), 0, 0, 0, 1),
       (900000102, 'qs:content_manager', '内容管理员', 'fangcun', 1, '问卷、量表和常模表的管理权限', NOW(), NOW(), 0, 0, 0, 1),
       (900000103, 'qs:evaluator', '评估员', 'fangcun', 1, '测评相关只读权限', NOW(), NOW(), 0, 0, 0, 1),
       (900000104, 'qs:staff', '普通员工', 'fangcun', 1, '基本查看权限', NOW(), NOW(), 0, 0, 0, 1),
       (900000105, 'qs:evaluation_plan_manager', '测评计划管理员', 'fangcun', 1, '测评计划的管理权限', NOW(), NOW(), 0, 0, 0,
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
VALUES (901000001, 'iam:identity:instance:profile', '个人资料', 'iam', 'identity', 'instance', JSON_ARRAY('read', 'update'),
        '当前用户自服务资料读取与更新', NOW(), NOW(), 0, 0, 0, 1),
       (901000002, 'iam:identity:collection:users', '用户管理', 'iam', 'identity', 'collection',
        JSON_ARRAY('read', 'search', 'create', 'update', 'deactivate', 'block', 'link_external_identity'),
        '用户资料、状态和外部身份关联管理', NOW(), NOW(), 0, 0, 0, 1),
       (901000003, 'iam:identity:collection:profiles', '档案', 'iam', 'identity', 'collection',
        JSON_ARRAY('read', 'list', 'search', 'create', 'update', 'search_by_mobile'), '档案查询、注册与更新', NOW(), NOW(), 0, 0, 0, 1),
       (901000004, 'iam:identity:collection:profile-links', '档案关系', 'iam', 'identity', 'collection',
        JSON_ARRAY('read', 'list', 'grant', 'update_relation', 'revoke', 'bulk_revoke', 'import'),
        '档案关系授予、更新、撤销与导入', NOW(), NOW(), 0, 0, 0, 1),
       (901000005, 'iam:authz:collection:roles', '角色管理', 'iam', 'authz', 'collection',
        JSON_ARRAY('create', 'read', 'update', 'delete', 'list'), '角色目录管理', NOW(), NOW(), 0, 0, 0, 1),
       (901000006, 'iam:authz:collection:assignments', '角色分配', 'iam', 'authz', 'collection',
        JSON_ARRAY('grant', 'revoke', 'delete', 'read'), '主体与角色分配管理', NOW(), NOW(), 0, 0, 0, 1),
       (901000007, 'iam:authz:collection:policies', '策略管理', 'iam', 'authz', 'collection',
        JSON_ARRAY('read', 'write', 'delete'), 'Casbin 策略规则管理', NOW(), NOW(), 0, 0, 0, 1),
       (901000008, 'iam:authz:collection:resources', '资源目录', 'iam', 'authz', 'collection',
        JSON_ARRAY('create', 'read', 'update', 'delete', 'list', 'validate_action'), '资源目录定义和动作校验', NOW(),
        NOW(), 0, 0, 0, 1),
       (901000009, 'iam:authz:action:check', '权限判定', 'iam', 'authz', 'action', JSON_ARRAY('check'), '单次 PDP 权限判定', NOW(),
        NOW(), 0, 0, 0, 1),
       (901000010, 'iam:authn:collection:login_identities', '登录身份管理', 'iam', 'authn', 'collection',
        JSON_ARRAY('read', 'update', 'enable', 'disable', 'set_unionid'), '登录身份读取、资料更新与启停用', NOW(), NOW(),
        0, 0, 0, 1),
       (901000011, 'iam:authn:collection:jwks', 'JWKS 密钥管理', 'iam', 'authn', 'collection',
        JSON_ARRAY('create', 'read', 'list', 'retire', 'force_retire', 'enter_grace', 'cleanup', 'list_publishable'),
        'JWT 签名密钥与发布管理', NOW(), NOW(), 0, 0, 0, 1),
       (901000012, 'iam:idp:collection:wechat_apps', '微信应用管理', 'iam', 'idp', 'collection',
        JSON_ARRAY('create', 'read', 'rotate_auth_secret', 'rotate_msg_secret', 'refresh_access_token',
                   'get_access_token'),
        '微信应用与令牌管理', NOW(), NOW(), 0, 0, 0, 1),
       (901000013, 'qs:questionnaire:collection:questionnaires', '问卷管理', 'qs', 'questionnaire', 'collection',
        JSON_ARRAY('create', 'read', 'list', 'update', 'delete', 'publish', 'unpublish', 'archive', 'statistics'),
        '问卷创建、维护、发布与统计', NOW(), NOW(), 0, 0, 0, 1),
       (901000014, 'qs:scale:collection:scales', '量表管理', 'qs', 'scale', 'collection',
        JSON_ARRAY('create', 'read', 'list', 'update', 'delete', 'publish', 'unpublish', 'archive'),
        '量表创建、维护与发布', NOW(), NOW(), 0, 0, 0, 1),
       (901000015, 'qs:answersheet:collection:answersheets', '答卷管理', 'qs', 'answersheet', 'collection',
        JSON_ARRAY('read', 'list', 'statistics', 'admin_submit'), '答卷查询、统计与管理员提交', NOW(), NOW(), 0, 0, 0,
        1),
       (901000016, 'qs:evaluation:collection:assessments', '测评执行', 'qs', 'evaluation', 'collection',
        JSON_ARRAY('read', 'list', 'retry', 'force_retry', 'batch_evaluate', 'statistics'), '测评任务、结果重试与批量执行', NOW(), NOW(),
        0, 0, 0, 1),
       (901000017, 'qs:evaluation:collection:reports', '测评报告', 'qs', 'evaluation', 'collection', JSON_ARRAY('read', 'list'),
        '测评报告查询', NOW(), NOW(), 0, 0, 0, 1),
       (901000018, 'qs:actor:collection:testees', '受试者管理', 'qs', 'actor', 'collection',
        JSON_ARRAY('read', 'list', 'update', 'analyze', 'statistics'), '受试者资料、分析与统计', NOW(), NOW(), 0, 0, 0,
        1),
       (901000019, 'qs:actor:collection:staff', '员工管理', 'qs', 'actor', 'collection',
        JSON_ARRAY('create', 'read', 'list', 'delete'), '员工创建、查询与删除', NOW(), NOW(), 0, 0, 0, 1),
       (901000020, 'qs:plan:collection:evaluation_plans', '测评计划', 'qs', 'plan', 'collection',
        JSON_ARRAY('create', 'read', 'list', 'update', 'pause', 'resume', 'cancel', 'enroll', 'terminate',
                   'statistics'),
        '测评计划生命周期与统计', NOW(), NOW(), 0, 0, 0, 1),
       (901000021, 'qs:plan_task:collection:evaluation_plan_tasks', '测评计划任务', 'qs', 'plan_task', 'collection',
        JSON_ARRAY('schedule', 'read', 'list', 'open', 'complete', 'expire', 'cancel'),
        '测评计划任务调度与状态流转', NOW(), NOW(), 0, 0, 0, 1),
       (901000022, 'qs:statistics:collection:system_statistics', '系统统计', 'qs', 'statistics', 'collection', JSON_ARRAY('read'),
        '后台系统统计查询', NOW(), NOW(), 0, 0, 0, 1),
       (901000023, 'qs:statistics:collection:statistics_jobs', '统计作业', 'qs', 'statistics', 'collection',
        JSON_ARRAY('sync', 'validate'), '统计同步与一致性校验', NOW(), NOW(), 0, 0, 0, 1),
       (901000024, 'qs:code:collection:codes', '邀请码申请', 'qs', 'code', 'collection', JSON_ARRAY('apply'), '邀请码申请', NOW(), NOW(),
        0, 0, 0, 1),
       (901000025, 'qs:modelcatalog:collection:norm_tables', '常模表管理', 'qs', 'modelcatalog', 'collection',
        JSON_ARRAY('read', 'list', 'import'), '版本化常模表的查询、详情读取与幂等导入', NOW(), NOW(), 0, 0, 0, 1)
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
      SELECT 902000003, 'user', '10001', 900000101, 'fangcun', 'system', NOW(), NOW(), NOW(), NULL, 0, 0, 0, 1
      UNION ALL
      SELECT 902000004, 'user', '110001', 2, 'fangcun', 'system', NOW(), NOW(), NOW(), NULL, 0, 0, 0, 1
      UNION ALL
      SELECT 902000005, 'user', '110001', 900000101, 'fangcun', 'system', NOW(), NOW(), NOW(), NULL, 0, 0, 0, 1
      UNION ALL
      SELECT 902000006, 'user', '110002', 900000102, 'fangcun', 'system', NOW(), NOW(), NOW(), NULL, 0, 0, 0, 1) AS `seed`
WHERE NOT EXISTS(SELECT 1
                 FROM `authz_assignments` `a`
                 WHERE `a`.`subject_type` = `seed`.`subject_type`
                   AND `a`.`subject_id` = `seed`.`subject_id`
                   AND `a`.`role_id` = `seed`.`role_id`
                   AND `a`.`tenant_id` = `seed`.`tenant_id`
                   AND `a`.`deleted_at` IS NULL);

-- ----------------------------------------------------------------------------
-- Historical Casbin policy baseline (retained as a non-executable migration
-- reference only; the final bootstrap below writes native facts).
-- ----------------------------------------------------------------------------

-- ----------------------------------------------------------------------------
-- Native authorization baseline
-- ----------------------------------------------------------------------------
UPDATE `authz_resources`
SET `attribute_schema` = JSON_OBJECT(
        'version', 1,
        'attributes', JSON_ARRAY(JSON_OBJECT(
            'key', 'object.origin_type',
            'type', 'string',
            'allowed_string_values', JSON_ARRAY('adhoc', 'plan')
        ))
    ),
    `updated_at` = NOW()
WHERE `key` = 'qs:evaluation:collection:assessments'
  AND `deleted_at` IS NULL;

INSERT INTO `authz_role_inheritances`
    (`id`, `tenant_id`, `role_id`, `inherited_role_id`, `granted_by`, `granted_at`,
     `created_at`, `updated_at`, `created_by`, `updated_by`, `deleted_by`, `version`)
SELECT `seed`.`id`, `seed`.`tenant_id`, `child`.`id`, `parent`.`id`, 'bootstrap', NOW(),
       NOW(), NOW(), 0, 0, 0, 1
FROM (
    SELECT 905000001 AS `id`, 'fangcun' AS `tenant_id`, 'tenant_admin' AS `child_role`, 'user' AS `parent_role`
    UNION ALL SELECT 905000002, 'fangcun', 'qs:admin', 'qs:content_manager'
    UNION ALL SELECT 905000003, 'fangcun', 'qs:admin', 'qs:evaluator'
    UNION ALL SELECT 905000004, 'fangcun', 'qs:admin', 'qs:evaluation_plan_manager'
    UNION ALL SELECT 905000005, 'fangcun', 'qs:evaluator', 'qs:staff'
    UNION ALL SELECT 905000006, 'fangcun', 'qs:evaluation_plan_manager', 'qs:staff'
) AS `seed`
JOIN `authz_roles` AS `child`
  ON `child`.`tenant_id` = `seed`.`tenant_id`
 AND `child`.`name` = `seed`.`child_role`
 AND `child`.`deleted_at` IS NULL
JOIN `authz_roles` AS `parent`
  ON `parent`.`tenant_id` = `seed`.`tenant_id`
 AND `parent`.`name` = `seed`.`parent_role`
 AND `parent`.`deleted_at` IS NULL
WHERE NOT EXISTS (
    SELECT 1
    FROM `authz_role_inheritances` AS `existing`
    WHERE `existing`.`tenant_id` = `seed`.`tenant_id`
      AND `existing`.`role_id` = `child`.`id`
      AND `existing`.`inherited_role_id` = `parent`.`id`
      AND `existing`.`revoked_at` IS NULL
      AND `existing`.`deleted_at` IS NULL
);

INSERT INTO `authz_permission_grants`
    (`id`, `tenant_id`, `role_id`, `resource_id`, `resource_pattern`, `action`, `constraint_set`,
     `grant_key`, `granted_by`, `granted_at`, `created_at`, `updated_at`, `created_by`, `updated_by`,
     `deleted_by`, `version`)
SELECT 1000000000000000000 + CAST(CONV(SUBSTRING(`resolved`.`grant_key`, 1, 14), 16, 10) AS UNSIGNED),
       `resolved`.`tenant_id`, `resolved`.`role_id`, `resolved`.`resource_id`, `resolved`.`resource_pattern`,
       `resolved`.`action`, `resolved`.`constraint_set`, `resolved`.`grant_key`, 'bootstrap', NOW(), NOW(), NOW(),
       0, 0, 0, 1
FROM (
    SELECT `expanded`.`tenant_id`, `role`.`id` AS `role_id`, `catalog`.`id` AS `resource_id`,
           `expanded`.`resource_pattern`, `expanded`.`action`, `expanded`.`constraint_set`,
           LOWER(SHA2(CONCAT(
               'v1', CHAR(0), `expanded`.`tenant_id`, CHAR(0), `role`.`id`, CHAR(0),
               COALESCE(`catalog`.`id`, 0), CHAR(0), `expanded`.`resource_pattern`, CHAR(0),
               `expanded`.`action`, CHAR(0), `expanded`.`constraint_set`
           ), 256)) AS `grant_key`
    FROM (
        SELECT `raw`.`tenant_id`, `raw`.`role_name`, `raw`.`resource_pattern`,
               `action_rows`.`action`, `raw`.`constraint_set`
        FROM (
            SELECT 'platform' AS `tenant_id`, 'super_admin' AS `role_name`, '*:*:*:*' AS `resource_pattern`,
                   JSON_ARRAY('*') AS `actions`, '{"version":1,"all_of":[]}' AS `constraint_set`
            UNION ALL SELECT 'fangcun', 'tenant_admin', 'iam:identity:collection:users',
                   JSON_ARRAY('read','search','create','update','deactivate','block','link_external_identity'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'tenant_admin', 'iam:identity:collection:profiles',
                   JSON_ARRAY('read','list','search','create','update','search_by_mobile'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'super_admin', 'iam:identity:collection:profiles',
                   JSON_ARRAY('search_by_mobile'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'tenant_admin', 'iam:identity:collection:profile-links',
                   JSON_ARRAY('read','list','grant','update_relation','revoke','bulk_revoke','import'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'tenant_admin', 'iam:authz:collection:roles',
                   JSON_ARRAY('create','read','update','delete','list'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'tenant_admin', 'iam:authz:collection:assignments',
                   JSON_ARRAY('grant','revoke','delete','read'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'tenant_admin', 'iam:authz:collection:policies',
                   JSON_ARRAY('read','write','delete'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'tenant_admin', 'iam:authz:collection:resources',
                   JSON_ARRAY('create','read','update','delete','list','validate_action'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'tenant_admin', 'iam:authz:action:check',
                   JSON_ARRAY('check'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'tenant_admin', 'iam:authn:collection:login_identities',
                   JSON_ARRAY('read','update','enable','disable'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'user', 'iam:identity:instance:profile',
                   JSON_ARRAY('read','update'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'qs:admin', 'qs:*:*:*',
                   JSON_ARRAY('*'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'qs:content_manager', 'qs:questionnaire:collection:questionnaires',
                   JSON_ARRAY('create','read','list','update','delete','publish','unpublish','archive','statistics'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'qs:content_manager', 'qs:scale:collection:scales',
                   JSON_ARRAY('create','read','list','update','delete','publish','unpublish','archive'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'qs:content_manager', 'qs:modelcatalog:collection:norm_tables',
                   JSON_ARRAY('read','list','import'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'qs:evaluator', 'qs:answersheet:collection:answersheets',
                   JSON_ARRAY('read','list','statistics'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'qs:evaluator', 'qs:evaluation:collection:assessments',
                   JSON_ARRAY('read','list','batch_evaluate','statistics'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'qs:evaluator', 'qs:evaluation:collection:assessments',
                   JSON_ARRAY('retry'), '{"version":1,"all_of":[{"key":"object.origin_type","operator":"eq","value":{"type":"string","string":"adhoc"}}]}'
            UNION ALL SELECT 'fangcun', 'qs:evaluator', 'qs:evaluation:collection:reports',
                   JSON_ARRAY('read','list'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'qs:evaluator', 'qs:actor:collection:testees',
                   JSON_ARRAY('read','list','analyze','statistics'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'qs:staff', 'qs:actor:collection:testees',
                   JSON_ARRAY('read','list'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'qs:evaluation_plan_manager', 'qs:plan:collection:evaluation_plans',
                   JSON_ARRAY('create','read','list','update','pause','resume','cancel','enroll','terminate','statistics'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'qs:evaluation_plan_manager', 'qs:plan_task:collection:evaluation_plan_tasks',
                   JSON_ARRAY('schedule','read','list','open','complete','expire','cancel'), '{"version":1,"all_of":[]}'
            UNION ALL SELECT 'fangcun', 'qs:evaluation_plan_manager', 'qs:evaluation:collection:assessments',
                   JSON_ARRAY('retry'), '{"version":1,"all_of":[{"key":"object.origin_type","operator":"eq","value":{"type":"string","string":"plan"}}]}'
        ) AS `raw`
        JOIN JSON_TABLE(`raw`.`actions`, '$[*]' COLUMNS (`action` VARCHAR(64) PATH '$')) AS `action_rows`
    ) AS `expanded`
    JOIN `authz_roles` AS `role`
      ON `role`.`tenant_id` = `expanded`.`tenant_id`
     AND `role`.`name` = `expanded`.`role_name`
     AND `role`.`deleted_at` IS NULL
    LEFT JOIN `authz_resources` AS `catalog`
      ON `catalog`.`key` = `expanded`.`resource_pattern`
     AND `catalog`.`deleted_at` IS NULL
) AS `resolved`
WHERE NOT EXISTS (
    SELECT 1
    FROM `authz_permission_grants` AS `existing`
    WHERE `existing`.`grant_key` = `resolved`.`grant_key`
      AND `existing`.`revoked_at` IS NULL
      AND `existing`.`deleted_at` IS NULL
);

-- ----------------------------------------------------------------------------
-- Policy versions
-- ----------------------------------------------------------------------------
INSERT INTO `authz_policy_versions` (`id`, `tenant_id`, `policy_version`, `changed_by`, `reason`, `created_at`,
                                     `updated_at`, `deleted_at`, `created_by`, `updated_by`, `deleted_by`, `version`)
VALUES (903000001, 'platform', 1, 'bootstrap', 'bootstrap baseline', NOW(), NOW(), NULL, 0, 0, 0, 1),
       (903000002, 'fangcun', 1, 'bootstrap', 'bootstrap baseline', NOW(), NOW(), NULL, 0, 0, 0, 1)
ON DUPLICATE KEY UPDATE `changed_by` = VALUES(`changed_by`),
                        `reason`     = VALUES(`reason`),
                        `deleted_at` = NULL,
                        `deleted_by` = 0,
                        `updated_at` = NOW(),
                        `updated_by` = 0;
