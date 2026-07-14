DELETE FROM `casbin_rule`
WHERE `ptype` = 'p'
  AND `v0` = 'role:qs:content_manager'
  AND `v1` = 'fangcun'
  AND `v2` = 'qs:modelcatalog:collection:norm_tables'
  AND `v3` = 'read|list|import'
  AND COALESCE(`v4`, '') = 'all:*';

DELETE FROM `authz_resources`
WHERE `id` = 901000025
  AND `key` = 'qs:modelcatalog:collection:norm_tables';

UPDATE `authz_roles`
SET `description` = '问卷和量表的管理权限',
    `updated_at` = NOW()
WHERE `name` = 'qs:content_manager'
  AND `tenant_id` = 'fangcun'
  AND `deleted_at` IS NULL;
