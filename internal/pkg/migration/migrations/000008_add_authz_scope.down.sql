ALTER TABLE `authz_resources`
    DROP COLUMN `scope_kinds`;

UPDATE `casbin_rule`
SET `v4` = NULL
WHERE `ptype` = 'p'
  AND `v4` = 'all:*';
