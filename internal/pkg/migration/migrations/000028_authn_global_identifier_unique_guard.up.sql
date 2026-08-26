-- global_identifier is a provider-level cross-realm lookup anchor. Exactly one
-- LoginIdentity row stores each (provider, global_identifier); additional
-- realm-specific rows for the same User keep the column NULL.

SET @iam_authn_global_identifier_invalid = (
    SELECT COUNT(*)
    FROM `auth_login_identities`
    WHERE `global_identifier` IS NOT NULL
      AND (
        TRIM(`global_identifier`) = ''
        OR BINARY `global_identifier` <> BINARY TRIM(`global_identifier`)
        OR TRIM(`provider`) = ''
        OR BINARY `provider` <> BINARY TRIM(`provider`)
        OR `user_id` = 0
      )
);
SET @iam_authn_global_identifier_conflicts = (
    SELECT COUNT(*)
    FROM (
        SELECT `provider`, `global_identifier`
        FROM `auth_login_identities`
        WHERE `global_identifier` IS NOT NULL
          AND TRIM(`global_identifier`) <> ''
        GROUP BY `provider`, `global_identifier`
        HAVING COUNT(DISTINCT `user_id`) > 1
    ) AS `conflicts`
);

DROP TEMPORARY TABLE IF EXISTS `iam_authn_global_identifier_assertion`;
CREATE TEMPORARY TABLE `iam_authn_global_identifier_assertion`
(
    `message` VARCHAR(128) NOT NULL PRIMARY KEY
);
INSERT INTO `iam_authn_global_identifier_assertion` (`message`)
VALUES ('AuthN global identifier preflight failed');
INSERT INTO `iam_authn_global_identifier_assertion` (`message`)
SELECT 'AuthN global identifier preflight failed'
WHERE @iam_authn_global_identifier_invalid <> 0
   OR @iam_authn_global_identifier_conflicts <> 0;
DROP TEMPORARY TABLE `iam_authn_global_identifier_assertion`;

-- Historical same-User duplicates are redundant. Keep one deterministic
-- canonical row, preferring an active identity and then the oldest binding.
DROP TEMPORARY TABLE IF EXISTS `iam_authn_global_identifier_duplicates`;
CREATE TEMPORARY TABLE `iam_authn_global_identifier_duplicates`
(
    `id` BIGINT UNSIGNED NOT NULL PRIMARY KEY
);
INSERT INTO `iam_authn_global_identifier_duplicates` (`id`)
SELECT `id`
FROM (
    SELECT
        `id`,
        ROW_NUMBER() OVER (
            PARTITION BY `provider`, `global_identifier`
            ORDER BY
                CASE WHEN `status` = 'active' THEN 0 ELSE 1 END,
                `linked_at` ASC,
                `created_at` ASC,
                `id` ASC
        ) AS `canonical_rank`
    FROM `auth_login_identities`
    WHERE `global_identifier` IS NOT NULL
) AS `ranked`
WHERE `canonical_rank` > 1;

UPDATE `auth_login_identities` AS `identity`
INNER JOIN `iam_authn_global_identifier_duplicates` AS `duplicate`
    ON `duplicate`.`id` = `identity`.`id`
SET `identity`.`global_identifier` = NULL;
DROP TEMPORARY TABLE `iam_authn_global_identifier_duplicates`;

ALTER TABLE `auth_login_identities`
    ADD UNIQUE KEY `uk_auth_login_identities_global`
        (`provider`, `global_identifier`);
