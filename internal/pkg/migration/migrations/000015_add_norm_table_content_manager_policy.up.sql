-- Register versioned norm-table administration and grant it to QS content managers.

INSERT INTO `authz_resources` (`id`, `key`, `display_name`, `app_name`, `domain`, `type`, `actions`, `description`,
                               `created_at`, `updated_at`, `created_by`, `updated_by`, `deleted_by`, `version`)
VALUES (901000025, 'qs:modelcatalog:collection:norm_tables', '常模表管理', 'qs', 'modelcatalog', 'collection',
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

-- Permission facts are seeded by the final native bootstrap after migration
-- 000027. Existing installations already applied the historical Casbin row;
-- the offline cutover converts that persisted fact.

UPDATE `authz_roles`
SET `description` = '问卷、量表和常模表的管理权限',
    `updated_at` = NOW()
WHERE `name` = 'qs:content_manager'
  AND `tenant_id` = 'fangcun'
  AND `deleted_at` IS NULL;
