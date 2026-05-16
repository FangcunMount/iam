-- 为档案 suggest 手机号精确搜索补充 Casbin 动作与资源目录；并为租户管理员授予 search_by_mobile。

UPDATE `authz_resources`
SET `actions` = JSON_ARRAY('read', 'list', 'search', 'create', 'update', 'search_by_mobile'),
    `updated_at` = NOW()
WHERE `key` = 'iam:identity:collection:profiles'
  AND `id` = 901000003;

INSERT INTO `casbin_rule` (`ptype`, `v0`, `v1`, `v2`, `v3`, `v4`, `v5`)
SELECT 'p', 'role:tenant_admin', 'fangcun', 'iam:identity:collection:profiles', 'search_by_mobile', 'all:*', NULL
WHERE NOT EXISTS(
    SELECT 1
    FROM `casbin_rule` `r`
    WHERE `r`.`ptype` = 'p'
      AND `r`.`v0` = 'role:tenant_admin'
      AND `r`.`v1` = 'fangcun'
      AND `r`.`v2` = 'iam:identity:collection:profiles'
      AND `r`.`v3` = 'search_by_mobile'
);

-- 租户超级管理员（业务域 fangcun）若显式绑定 Casbin 角色 role:super_admin，则一并授予。
INSERT INTO `casbin_rule` (`ptype`, `v0`, `v1`, `v2`, `v3`, `v4`, `v5`)
SELECT 'p', 'role:super_admin', 'fangcun', 'iam:identity:collection:profiles', 'search_by_mobile', 'all:*', NULL
WHERE NOT EXISTS(
    SELECT 1
    FROM `casbin_rule` `r`
    WHERE `r`.`ptype` = 'p'
      AND `r`.`v0` = 'role:super_admin'
      AND `r`.`v1` = 'fangcun'
      AND `r`.`v2` = 'iam:identity:collection:profiles'
      AND `r`.`v3` = 'search_by_mobile'
);
