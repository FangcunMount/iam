-- Migration: bridge pre-000012 identity resource aliases and normalize legacy Casbin tenant domain "1".
-- 000014-v4: MySQL-safe (JOIN only, no same-table subquery DELETE), idempotent.

-- ---------------------------------------------------------------------------
-- 1) authz_resources: legacy alias keys -> four-segment keys
-- ---------------------------------------------------------------------------

DELETE `extra` FROM `authz_resources` AS `extra`
INNER JOIN `authz_resources` AS `keep`
    ON `extra`.`key` = `keep`.`key`
   AND `extra`.`id` > `keep`.`id`
WHERE `extra`.`key` IN ('iam:accounts', 'iam:children', 'iam:guardianships');

DELETE `legacy` FROM `authz_resources` AS `legacy`
INNER JOIN `authz_resources` AS `canonical`
    ON `canonical`.`id` <> `legacy`.`id`
   AND `canonical`.`key` = CASE `legacy`.`key`
       WHEN 'iam:accounts' THEN 'iam:authn:collection:login_identities'
       WHEN 'iam:children' THEN 'iam:identity:collection:profiles'
       WHEN 'iam:guardianships' THEN 'iam:identity:collection:profile-links'
       ELSE `legacy`.`key`
   END
WHERE `legacy`.`key` IN ('iam:accounts', 'iam:children', 'iam:guardianships');

UPDATE `authz_resources`
SET `key` = CASE `key`
    WHEN 'iam:accounts' THEN 'iam:authn:collection:login_identities'
    WHEN 'iam:children' THEN 'iam:identity:collection:profiles'
    WHEN 'iam:guardianships' THEN 'iam:identity:collection:profile-links'
    ELSE `key`
END
WHERE `key` IN ('iam:accounts', 'iam:children', 'iam:guardianships');

-- ---------------------------------------------------------------------------
-- 2) casbin_rule p-facts: legacy resource keys (domain unchanged)
-- ---------------------------------------------------------------------------

DELETE `extra` FROM `casbin_rule` AS `extra`
INNER JOIN `casbin_rule` AS `keep`
    ON `extra`.`ptype` = `keep`.`ptype`
   AND `extra`.`v0` = `keep`.`v0`
   AND `extra`.`v1` = `keep`.`v1`
   AND `extra`.`v2` = `keep`.`v2`
   AND `extra`.`v3` = `keep`.`v3`
   AND COALESCE(`extra`.`v4`, '') = COALESCE(`keep`.`v4`, '')
   AND COALESCE(`extra`.`v5`, '') = COALESCE(`keep`.`v5`, '')
   AND `extra`.`id` > `keep`.`id`
WHERE `extra`.`ptype` = 'p'
  AND `extra`.`v2` IN ('iam:accounts', 'iam:children', 'iam:guardianships');

DELETE `later` FROM `casbin_rule` AS `later`
INNER JOIN `casbin_rule` AS `earlier`
    ON `later`.`ptype` = 'p'
   AND `earlier`.`ptype` = 'p'
   AND `later`.`v0` = `earlier`.`v0`
   AND `later`.`v1` = `earlier`.`v1`
   AND `later`.`v3` = `earlier`.`v3`
   AND COALESCE(`later`.`v4`, '') = COALESCE(`earlier`.`v4`, '')
   AND COALESCE(`later`.`v5`, '') = COALESCE(`earlier`.`v5`, '')
   AND `later`.`id` > `earlier`.`id`
   AND `later`.`v2` IN ('iam:accounts', 'iam:children', 'iam:guardianships')
   AND `earlier`.`v2` IN ('iam:accounts', 'iam:children', 'iam:guardianships')
   AND CASE `later`.`v2`
       WHEN 'iam:accounts' THEN 'iam:authn:collection:login_identities'
       WHEN 'iam:children' THEN 'iam:identity:collection:profiles'
       WHEN 'iam:guardianships' THEN 'iam:identity:collection:profile-links'
       ELSE `later`.`v2`
   END = CASE `earlier`.`v2`
       WHEN 'iam:accounts' THEN 'iam:authn:collection:login_identities'
       WHEN 'iam:children' THEN 'iam:identity:collection:profiles'
       WHEN 'iam:guardianships' THEN 'iam:identity:collection:profile-links'
       ELSE `earlier`.`v2`
   END;

DELETE `legacy` FROM `casbin_rule` AS `legacy`
INNER JOIN `casbin_rule` AS `canonical`
    ON `canonical`.`ptype` = 'p'
   AND `canonical`.`id` <> `legacy`.`id`
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

-- ---------------------------------------------------------------------------
-- 3) tenant domain: legacy "1" -> "fangcun"
-- ---------------------------------------------------------------------------

DELETE `dup` FROM `authz_roles` AS `dup`
INNER JOIN `authz_roles` AS `keep`
    ON `keep`.`tenant_id` = 'fangcun'
   AND `keep`.`name` = `dup`.`name`
   AND `keep`.`id` <> `dup`.`id`
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
   AND `keep`.`id` <> `dup`.`id`
   AND `keep`.`deleted_at` IS NULL
WHERE `dup`.`tenant_id` = '1'
  AND `dup`.`deleted_at` IS NULL;

UPDATE `authz_assignments`
SET `tenant_id` = 'fangcun'
WHERE `tenant_id` = '1';

DELETE `old` FROM `authz_policy_versions` AS `old`
INNER JOIN `authz_policy_versions` AS `current`
    ON `current`.`tenant_id` = 'fangcun'
   AND `current`.`deleted_at` IS NULL
WHERE `old`.`tenant_id` = '1';

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
   AND `keep`.`id` <> `dup`.`id`
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
   AND `keep`.`id` <> `dup`.`id`
WHERE `dup`.`ptype` = 'g'
  AND `dup`.`v2` = '1';

UPDATE `casbin_rule`
SET `v2` = 'fangcun'
WHERE `ptype` = 'g'
  AND `v2` = '1';
