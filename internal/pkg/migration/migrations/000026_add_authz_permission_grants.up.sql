-- Expand the AuthZ management model. The new grant tables remain dormant: this
-- migration creates no grants, performs no Casbin backfill, and removes no
-- legacy Casbin fact. The exact compatibility repairs below only address rows
-- verified against the production backup clone.

CREATE TABLE IF NOT EXISTS `authz_permission_grants`
(
    `id`               BIGINT UNSIGNED NOT NULL PRIMARY KEY COMMENT 'PermissionGrant ID',
    `tenant_id`        VARCHAR(64)     NOT NULL COMMENT 'Tenant/domain ID',
    `role_id`          BIGINT UNSIGNED NOT NULL COMMENT 'Granted role ID',
    `resource_id`      BIGINT UNSIGNED          DEFAULT NULL COMMENT 'Exact catalog resource ID; NULL for trusted system wildcard',
    `resource_pattern` VARCHAR(128)    NOT NULL COMMENT 'Canonical four-segment resource pattern',
    `action`           VARCHAR(64)     NOT NULL COMMENT 'Concrete action or trusted * wildcard',
    `constraint_set`   JSON            NOT NULL COMMENT 'Versioned typed ConstraintSet',
    `grant_key`        CHAR(64)        NOT NULL COMMENT 'SHA-256 canonical grant identity',
    `granted_by`       VARCHAR(64)     NOT NULL COMMENT 'Grant operator/service',
    `granted_at`       DATETIME(3)     NOT NULL COMMENT 'Grant time',
    `revoked_at`       DATETIME(3)              DEFAULT NULL COMMENT 'Revocation time',
    `active_guard`     TINYINT GENERATED ALWAYS AS (CASE WHEN `revoked_at` IS NULL AND `deleted_at` IS NULL THEN 1 ELSE NULL END) STORED COMMENT 'Active grant uniqueness guard',
    `created_at`       DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    `updated_at`       DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    `deleted_at`       DATETIME(3)              DEFAULT NULL,
    `created_by`       BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `updated_by`       BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `deleted_by`       BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `version`          INT UNSIGNED    NOT NULL DEFAULT 1,
    UNIQUE KEY `uk_authz_permission_grants_active` (`grant_key`, `active_guard`),
    KEY `idx_authz_permission_grants_tenant_role` (`tenant_id`, `role_id`),
    KEY `idx_authz_permission_grants_resource_action` (`tenant_id`, `resource_pattern`, `action`),
    KEY `idx_authz_permission_grants_resource_id` (`resource_id`),
    KEY `idx_authz_permission_grants_deleted_at` (`deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='Typed AuthZ permission grants';

CREATE TABLE IF NOT EXISTS `authz_role_inheritances`
(
    `id`                BIGINT UNSIGNED NOT NULL PRIMARY KEY COMMENT 'Role inheritance ID',
    `tenant_id`         VARCHAR(64)     NOT NULL COMMENT 'Tenant/domain ID',
    `role_id`           BIGINT UNSIGNED NOT NULL COMMENT 'Role receiving inherited capabilities',
    `inherited_role_id` BIGINT UNSIGNED NOT NULL COMMENT 'Inherited parent role ID',
    `granted_by`        VARCHAR(64)     NOT NULL COMMENT 'Grant operator/service',
    `granted_at`        DATETIME(3)     NOT NULL COMMENT 'Grant time',
    `revoked_at`        DATETIME(3)              DEFAULT NULL COMMENT 'Revocation time',
    `active_guard`      TINYINT GENERATED ALWAYS AS (CASE WHEN `revoked_at` IS NULL AND `deleted_at` IS NULL THEN 1 ELSE NULL END) STORED COMMENT 'Active inheritance uniqueness guard',
    `created_at`        DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    `updated_at`        DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    `deleted_at`        DATETIME(3)              DEFAULT NULL,
    `created_by`        BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `updated_by`        BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `deleted_by`        BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `version`           INT UNSIGNED    NOT NULL DEFAULT 1,
    UNIQUE KEY `uk_authz_role_inheritances_active` (`tenant_id`, `role_id`, `inherited_role_id`, `active_guard`),
    KEY `idx_authz_role_inheritances_parent` (`tenant_id`, `inherited_role_id`),
    KEY `idx_authz_role_inheritances_deleted_at` (`deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='First-class role inheritance facts';

-- Single-row cutover guard. Only the offline maintenance tool may mark this
-- row verified; migration 000027 consumes the marker before retiring legacy
-- authorization storage.
CREATE TABLE IF NOT EXISTS `authz_cutover_state`
(
    `id`            TINYINT UNSIGNED NOT NULL PRIMARY KEY,
    `status`        VARCHAR(16)      NOT NULL,
    `evidence_hash` CHAR(64)         NOT NULL,
    `verified_at`   DATETIME(3)               DEFAULT NULL,
    `created_at`    DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    `updated_at`    DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    CONSTRAINT `chk_authz_cutover_state_singleton` CHECK (`id` = 1),
    CONSTRAINT `chk_authz_cutover_state_status` CHECK (`status` IN ('verified'))
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='Verified offline AuthZ cutover evidence';

SET @iam_has_attribute_schema = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'authz_resources'
      AND COLUMN_NAME = 'attribute_schema'
);
SET @iam_sql = IF(
    @iam_has_attribute_schema = 0,
    'ALTER TABLE `authz_resources` ADD COLUMN `attribute_schema` JSON NULL COMMENT ''Versioned object attribute schema'' AFTER `scope_kinds`',
    'SELECT 1'
);
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

-- Repair the one verified pre-four-segment catalog drift found in the
-- production backup. Migration 000012 canonicalized this resource key but the
-- historical row retained its old `uc` domain metadata. Keep the predicate
-- exact: any other key or metadata mismatch must remain visible to preflight.
UPDATE `authz_resources`
SET `domain` = 'identity',
    `updated_at` = CURRENT_TIMESTAMP(3),
    `updated_by` = 0
WHERE `key` = 'iam:identity:instance:profile'
  AND `app_name` = 'iam'
  AND `domain` = 'uc'
  AND `type` = 'instance'
  AND `deleted_at` IS NULL;

-- Retire six verified stale bootstrap assignments left behind when the
-- corresponding system roles were rebuilt with generated IDs. Each subject
-- already has the intended active replacement assignment. Keep every identity
-- in the predicate exact and require the old role to be absent; any drift must
-- remain visible to the offline cutover preflight instead of being guessed.
UPDATE `authz_assignments` AS `stale_assignment`
INNER JOIN (
    SELECT 902000001 AS `stale_assignment_id`, 'user' AS `subject_type`, '10001' AS `subject_id`,
           900000001 AS `stale_role_id`, 'platform' AS `tenant_id`, 'super_admin' AS `current_role_name`,
           613485615136125486 AS `current_role_id`, 613500091088515630 AS `current_assignment_id`
    UNION ALL
    SELECT 902000002, 'user', '10001', 2, 'fangcun', 'tenant_admin',
           613485615152902702, 613500091021406766
    UNION ALL
    SELECT 902000003, 'user', '10001', 900000101, 'fangcun', 'qs:admin',
           613485615186457134, 613500090887189038
    UNION ALL
    SELECT 902000004, 'user', '110001', 2, 'fangcun', 'tenant_admin',
           613485615152902702, 613486857019208238
    UNION ALL
    SELECT 902000005, 'user', '110001', 900000101, 'fangcun', 'qs:admin',
           613485615186457134, 613486857069539886
    UNION ALL
    SELECT 902000006, 'user', '110002', 900000102, 'fangcun', 'qs:content_manager',
           613485615203234350, 613500090954297902
) AS `verified_seed`
    ON `stale_assignment`.`id` = `verified_seed`.`stale_assignment_id`
   AND `stale_assignment`.`subject_type` = `verified_seed`.`subject_type`
   AND `stale_assignment`.`subject_id` = `verified_seed`.`subject_id`
   AND `stale_assignment`.`role_id` = `verified_seed`.`stale_role_id`
   AND `stale_assignment`.`tenant_id` = `verified_seed`.`tenant_id`
LEFT JOIN `authz_roles` AS `stale_role`
    ON `stale_role`.`id` = `verified_seed`.`stale_role_id`
   AND `stale_role`.`deleted_at` IS NULL
INNER JOIN `authz_roles` AS `current_role`
    ON `current_role`.`id` = `verified_seed`.`current_role_id`
   AND `current_role`.`name` = `verified_seed`.`current_role_name`
   AND `current_role`.`tenant_id` = `verified_seed`.`tenant_id`
   AND `current_role`.`deleted_at` IS NULL
INNER JOIN `authz_assignments` AS `current_assignment`
    ON `current_assignment`.`id` = `verified_seed`.`current_assignment_id`
   AND `current_assignment`.`subject_type` = `verified_seed`.`subject_type`
   AND `current_assignment`.`subject_id` = `verified_seed`.`subject_id`
   AND `current_assignment`.`role_id` = `verified_seed`.`current_role_id`
   AND `current_assignment`.`tenant_id` = `verified_seed`.`tenant_id`
   AND `current_assignment`.`deleted_at` IS NULL
SET `stale_assignment`.`deleted_at` = CURRENT_TIMESTAMP(3),
    `stale_assignment`.`updated_at` = CURRENT_TIMESTAMP(3),
    `stale_assignment`.`deleted_by` = 0,
    `stale_assignment`.`updated_by` = 0,
    `stale_assignment`.`version` = `stale_assignment`.`version` + 1
WHERE `stale_assignment`.`deleted_at` IS NULL
  AND `stale_role`.`id` IS NULL;

-- Register the first trusted object attribute contract before offline
-- conversion. The retry force action is catalogued separately from retry so
-- it cannot inherit the evaluator's conditional grant.
UPDATE `authz_resources`
SET `attribute_schema` = JSON_OBJECT(
        'version', 1,
        'attributes', JSON_ARRAY(JSON_OBJECT(
            'key', 'object.origin_type',
            'type', 'string',
            'allowed_string_values', JSON_ARRAY('adhoc', 'plan')
        ))
    ),
    `actions` = IF(
        JSON_CONTAINS(`actions`, JSON_QUOTE('force_retry')),
        `actions`,
        JSON_ARRAY_APPEND(`actions`, '$', 'force_retry')
    ),
    `updated_at` = CURRENT_TIMESTAMP(3)
WHERE `key` = 'qs:evaluation:collection:assessments'
  AND `deleted_at` IS NULL;
