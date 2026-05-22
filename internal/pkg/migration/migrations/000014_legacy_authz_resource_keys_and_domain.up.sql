-- Migration: bridge pre-000012 identity resource aliases and normalize legacy Casbin tenant domain "1".
-- Idempotent and conflict-safe for dirty-state recovery.

DELETE `legacy` FROM `authz_resources` AS `legacy`
WHERE `legacy`.`key` IN ('iam:accounts', 'iam:children', 'iam:guardianships')
  AND EXISTS (
      SELECT 1
      FROM `authz_resources` AS `canonical`
      WHERE `canonical`.`key` = CASE `legacy`.`key`
          WHEN 'iam:accounts' THEN 'iam:authn:collection:login_identities'
          WHEN 'iam:children' THEN 'iam:identity:collection:profiles'
          WHEN 'iam:guardianships' THEN 'iam:identity:collection:profile-links'
          ELSE `legacy`.`key`
      END
        AND `canonical`.`deleted_at` IS NULL
  );

UPDATE `authz_resources`
SET `key` = CASE `key`
    WHEN 'iam:accounts' THEN 'iam:authn:collection:login_identities'
    WHEN 'iam:children' THEN 'iam:identity:collection:profiles'
    WHEN 'iam:guardianships' THEN 'iam:identity:collection:profile-links'
    ELSE `key`
END
WHERE `key` IN ('iam:accounts', 'iam:children', 'iam:guardianships');

DELETE `legacy` FROM `casbin_rule` AS `legacy`
INNER JOIN `casbin_rule` AS `canonical`
    ON `canonical`.`ptype` = 'p'
   AND `canonical`.`v0` = `legacy`.`v0`
   AND `canonical`.`v1` = `legacy`.`v1`
   AND `canonical`.`v3` = `legacy`.`v3`
   AND COALESCE(`canonical`.`v4`, '') = COALESCE(`legacy`.`v4`, '')
   AND COALESCE(`canonical`.`v5`, '') = COALESCE(`legacy`.`v5`, '')
   AND `canonical`.`v2` = CASE `legacy`.`v2`
       WHEN 'iam:accounts' THEN 'iam:authn:collection:login_identities'
       WHEN 'iam:children' THEN 'iam:identity:collection:profiles'
       WHEN 'iam:guardianships' THEN 'iam:identity:collection:profile-links'
       ELSE `legacy`.`v2`
   END
WHERE `legacy`.`ptype` = 'p'
  AND `legacy`.`v2` IN ('iam:accounts', 'iam:children', 'iam:guardianships');

UPDATE `casbin_rule`
SET `v2` = CASE `v2`
    WHEN 'iam:accounts' THEN 'iam:authn:collection:login_identities'
    WHEN 'iam:children' THEN 'iam:identity:collection:profiles'
    WHEN 'iam:guardianships' THEN 'iam:identity:collection:profile-links'
    ELSE `v2`
END
WHERE `ptype` = 'p'
  AND `v2` IN ('iam:accounts', 'iam:children', 'iam:guardianships');

DELETE `dup` FROM `authz_roles` AS `dup`
INNER JOIN `authz_roles` AS `keep`
    ON `keep`.`tenant_id` = 'fangcun'
   AND `keep`.`name` = `dup`.`name`
   AND `keep`.`deleted_at` IS NULL
WHERE `dup`.`tenant_id` = '1'
  AND `dup`.`deleted_at` IS NULL;

UPDATE `authz_roles`
SET `tenant_id` = 'fangcun'
WHERE `tenant_id` = '1';

DELETE `dup` FROM `authz_assignments` AS `dup`
INNER JOIN `authz_assignments` AS `keep`
    ON `keep`.`tenant_id` = 'fangcun'
   AND `keep`.`subject_type` = `dup`.`subject_type`
   AND `keep`.`subject_id` = `dup`.`subject_id`
   AND `keep`.`role_id` = `dup`.`role_id`
   AND `keep`.`deleted_at` IS NULL
WHERE `dup`.`tenant_id` = '1'
  AND `dup`.`deleted_at` IS NULL;

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

DELETE `dup` FROM `casbin_rule` AS `dup`
INNER JOIN `casbin_rule` AS `keep`
    ON `keep`.`ptype` = 'p'
   AND `keep`.`v1` = 'fangcun'
   AND `keep`.`v0` = `dup`.`v0`
   AND `keep`.`v2` = `dup`.`v2`
   AND `keep`.`v3` = `dup`.`v3`
   AND COALESCE(`keep`.`v4`, '') = COALESCE(`dup`.`v4`, '')
   AND COALESCE(`keep`.`v5`, '') = COALESCE(`dup`.`v5`, '')
WHERE `dup`.`ptype` = 'p'
  AND `dup`.`v1` = '1';

UPDATE `casbin_rule`
SET `v1` = 'fangcun'
WHERE `ptype` = 'p'
  AND `v1` = '1';

DELETE `dup` FROM `casbin_rule` AS `dup`
INNER JOIN `casbin_rule` AS `keep`
    ON `keep`.`ptype` = 'g'
   AND `keep`.`v2` = 'fangcun'
   AND `keep`.`v0` = `dup`.`v0`
   AND `keep`.`v1` = `dup`.`v1`
   AND COALESCE(`keep`.`v3`, '') = COALESCE(`dup`.`v3`, '')
   AND COALESCE(`keep`.`v4`, '') = COALESCE(`dup`.`v4`, '')
   AND COALESCE(`keep`.`v5`, '') = COALESCE(`dup`.`v5`, '')
WHERE `dup`.`ptype` = 'g'
  AND `dup`.`v2` = '1';

UPDATE `casbin_rule`
SET `v2` = 'fangcun'
WHERE `ptype` = 'g'
  AND `v2` = '1';
