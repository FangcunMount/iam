-- 为档案 suggest 手机号精确搜索补充 Casbin 动作与资源目录；并为租户管理员授予 search_by_mobile。

UPDATE `authz_resources`
SET `actions` = JSON_ARRAY('read', 'list', 'search', 'create', 'update', 'search_by_mobile'),
    `updated_at` = NOW()
WHERE `key` = 'iam:identity:collection:profiles'
  AND `id` = 901000003;

-- Permission facts are seeded by the final native bootstrap after migration
-- 000027. Existing installations already applied the historical Casbin rows;
-- the offline cutover converts those persisted facts.
