-- Migration: bridge pre-000012 identity resource aliases and normalize legacy Casbin tenant domain "1".
-- Fixes GetAuthorizationSnapshot InvalidArgument when tenant_admin still carries iam:accounts|children|guardianships.

UPDATE `authz_resources`
SET `key` = CASE `key`
    WHEN 'iam:accounts' THEN 'iam:authn:collection:login_identities'
    WHEN 'iam:children' THEN 'iam:identity:collection:profiles'
    WHEN 'iam:guardianships' THEN 'iam:identity:collection:profile-links'
    ELSE `key`
END
WHERE `key` IN ('iam:accounts', 'iam:children', 'iam:guardianships');

UPDATE `casbin_rule`
SET `v2` = CASE `v2`
    WHEN 'iam:accounts' THEN 'iam:authn:collection:login_identities'
    WHEN 'iam:children' THEN 'iam:identity:collection:profiles'
    WHEN 'iam:guardianships' THEN 'iam:identity:collection:profile-links'
    ELSE `v2`
END
WHERE `ptype` = 'p'
  AND `v2` IN ('iam:accounts', 'iam:children', 'iam:guardianships');

UPDATE `authz_roles`
SET `tenant_id` = 'fangcun'
WHERE `tenant_id` = '1';

UPDATE `authz_assignments`
SET `tenant_id` = 'fangcun'
WHERE `tenant_id` = '1';

DELETE FROM `authz_policy_versions`
WHERE `tenant_id` = '1'
  AND EXISTS (
      SELECT 1
      FROM (SELECT 1 FROM `authz_policy_versions` WHERE `tenant_id` = 'fangcun' AND `deleted_at` IS NULL) `existing`
  );

UPDATE `authz_policy_versions`
SET `tenant_id` = 'fangcun'
WHERE `tenant_id` = '1';

UPDATE `casbin_rule`
SET `v1` = 'fangcun'
WHERE `ptype` = 'p'
  AND `v1` = '1';

UPDATE `casbin_rule`
SET `v2` = 'fangcun'
WHERE `ptype` = 'g'
  AND `v2` = '1';
