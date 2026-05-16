-- 回滚 000013：移除 suggest 手机号搜索策略，还原档案资源动作列表。

DELETE FROM `casbin_rule`
WHERE `ptype` = 'p'
  AND `v1` = 'fangcun'
  AND `v2` = 'iam:identity:collection:profiles'
  AND `v3` = 'search_by_mobile'
  AND `v0` IN ('role:tenant_admin', 'role:super_admin');

UPDATE `authz_resources`
SET `actions` = JSON_ARRAY('read', 'list', 'search', 'create', 'update'),
    `updated_at` = NOW()
WHERE `key` = 'iam:identity:collection:profiles'
  AND `id` = 901000003;
