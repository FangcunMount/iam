ALTER TABLE `authz_resources`
    ADD COLUMN `scope_kinds` TEXT NULL COMMENT '资源支持的授权范围类型 (JSON数组格式)' AFTER `actions`;

UPDATE `authz_resources`
SET `scope_kinds` = JSON_ARRAY('all')
WHERE `scope_kinds` IS NULL
   OR `scope_kinds` = '';

UPDATE `casbin_rule`
SET `v4` = 'all:*'
WHERE `ptype` = 'p'
  AND (`v4` IS NULL OR `v4` = '');
