UPDATE `casbin_rule`
SET `v1` = '1'
WHERE `ptype` = 'p'
  AND `v1` = 'fangcun'
  AND (`v0` LIKE 'role:qs:%' OR `v2` LIKE 'qs:%');

UPDATE `casbin_rule`
SET `v2` = '1'
WHERE `ptype` = 'g'
  AND `v2` = 'fangcun'
  AND (`v0` LIKE 'role:qs:%' OR `v1` LIKE 'role:qs:%' OR (`v0` LIKE 'user:%' AND `v1` LIKE 'role:qs:%'));

INSERT INTO `authz_policy_versions` (`id`, `tenant_id`, `policy_version`, `changed_by`, `reason`, `created_at`,
                                     `updated_at`, `deleted_at`, `created_by`, `updated_by`, `deleted_by`, `version`)
SELECT 903000003, '1', 1, 'bootstrap', 'bootstrap baseline', NOW(), NOW(), NULL, 0, 0, 0, 1
WHERE NOT EXISTS (
    SELECT 1 FROM `authz_policy_versions` WHERE `tenant_id` = '1' AND `policy_version` = 1 AND `deleted_at` IS NULL
);

UPDATE `authz_assignments`
SET `tenant_id` = '1'
WHERE `tenant_id` = 'fangcun'
  AND `role_id` IN (900000101, 900000102, 900000103, 900000104, 900000105);

UPDATE `authz_roles`
SET `tenant_id` = '1'
WHERE `tenant_id` = 'fangcun'
  AND `id` IN (900000101, 900000102, 900000103, 900000104, 900000105);

UPDATE `casbin_rule`
SET `v2` = CASE `v2`
    WHEN 'iam:authn:collection:login_identities' THEN 'iam:accounts'
    WHEN 'iam:identity:collection:profiles' THEN 'iam:children'
    WHEN 'iam:identity:collection:profile-links' THEN 'iam:guardianships'
    ELSE `v2`
END
WHERE `ptype` = 'p'
  AND `v2` IN (
    'iam:authn:collection:login_identities',
    'iam:identity:collection:profiles',
    'iam:identity:collection:profile-links'
  );

UPDATE `authz_resources`
SET `key` = CASE `key`
    WHEN 'iam:authn:collection:login_identities' THEN 'iam:accounts'
    WHEN 'iam:identity:collection:profiles' THEN 'iam:children'
    WHEN 'iam:identity:collection:profile-links' THEN 'iam:guardianships'
    ELSE `key`
END
WHERE `key` IN (
    'iam:authn:collection:login_identities',
    'iam:identity:collection:profiles',
    'iam:identity:collection:profile-links'
  );
