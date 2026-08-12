-- Retire the Account-bound AuthN compatibility tables. The canonical runtime
-- contract is auth_login_identities + auth_credentials; format-v5 production
-- reconciliation must already have converged before this migration is shipped.
-- All assertions execute before the single final DROP so a failed gate leaves
-- both legacy tables available for diagnosis and backup-based recovery.

SET @iam_authn_legacy_table_count = (
    SELECT COUNT(*)
    FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME IN ('auth_accounts', 'auth_credentials_legacy')
);
SET @iam_authn_canonical_table_count = (
    SELECT COUNT(*)
    FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME IN ('auth_login_identities', 'auth_credentials')
);
SET @iam_authn_canonical_column_count = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND (
        (TABLE_NAME = 'auth_login_identities'
          AND COLUMN_NAME IN ('id', 'user_id', 'provider', 'realm', 'identifier', 'global_identifier'))
        OR
        (TABLE_NAME = 'auth_credentials'
          AND COLUMN_NAME IN ('id', 'login_identity_id', 'type', 'material', 'algo'))
      )
);
SET @iam_authn_old_credential_shape = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'auth_credentials'
      AND COLUMN_NAME = 'account_id'
);
SET @iam_authn_legacy_column_count = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND (
        (TABLE_NAME = 'auth_accounts'
          AND COLUMN_NAME IN ('id', 'user_id', 'type', 'app_id', 'external_id', 'unique_id', 'scoped_tenant_id', 'status'))
        OR
        (TABLE_NAME = 'auth_credentials_legacy'
          AND COLUMN_NAME IN ('id', 'account_id', 'type', 'idp', 'idp_identifier', 'app_id', 'material', 'algo', 'status'))
      )
);

DROP TEMPORARY TABLE IF EXISTS iam_authn_schema_assertion;
CREATE TEMPORARY TABLE iam_authn_schema_assertion
(
    message VARCHAR(128) NOT NULL PRIMARY KEY
);
INSERT INTO iam_authn_schema_assertion (message)
VALUES ('canonical AuthN schema or legacy table pair is incomplete');
INSERT INTO iam_authn_schema_assertion (message)
SELECT 'canonical AuthN schema or legacy table pair is incomplete'
WHERE @iam_authn_canonical_table_count <> 2
   OR @iam_authn_canonical_column_count <> 11
   OR @iam_authn_old_credential_shape <> 0
   OR @iam_authn_legacy_table_count NOT IN (0, 2)
   OR (@iam_authn_legacy_table_count = 2 AND @iam_authn_legacy_column_count <> 17);
DROP TEMPORARY TABLE iam_authn_schema_assertion;

-- Every supported Account must resolve to the same immutable canonical owner.
-- Mutable status differences are intentionally not copied back from legacy.
SET @iam_authn_account_blockers = 0;
SET @iam_sql = IF(
    @iam_authn_legacy_table_count = 2,
    'WITH account_keys AS (
       SELECT a.*,
         a.type IN (''opera'', ''mock-consumer'', ''wc-minip'', ''wc-com'') AS supported,
         CASE a.type WHEN ''wc-minip'' THEN ''wechat_minip'' WHEN ''wc-com'' THEN ''wecom'' ELSE ''username'' END AS expected_provider,
         CASE WHEN a.type = ''opera'' AND a.scoped_tenant_id <> 0 THEN CAST(a.scoped_tenant_id AS CHAR)
              WHEN a.type IN (''wc-minip'', ''wc-com'') THEN TRIM(a.app_id) ELSE ''default'' END AS expected_realm,
         TRIM(a.external_id) AS expected_identifier,
         NULLIF(TRIM(COALESCE(a.unique_id, '''')), '''') AS expected_global_identifier
       FROM auth_accounts a
     ), account_facts AS (
       SELECT ak.*,
         COUNT(*) OVER (
           PARTITION BY CAST(ak.expected_provider AS BINARY), CAST(ak.expected_realm AS BINARY),
                        CAST(ak.expected_identifier AS BINARY)
         ) AS source_key_count,
         (SELECT COUNT(*) FROM auth_login_identities li
          WHERE CAST(li.provider AS BINARY) = CAST(ak.expected_provider AS BINARY)
            AND CAST(li.realm AS BINARY) = CAST(ak.expected_realm AS BINARY)
            AND CAST(li.identifier AS BINARY) = CAST(ak.expected_identifier AS BINARY)
            AND li.user_id = ak.user_id
            AND (ak.expected_global_identifier IS NULL
              OR CAST(li.global_identifier AS BINARY) = CAST(ak.expected_global_identifier AS BINARY))) AS exact_mapping_count
       FROM account_keys ak
     )
     SELECT COUNT(*) INTO @iam_authn_account_blockers
     FROM account_facts
     WHERE NOT supported
        OR user_id = 0
        OR OCTET_LENGTH(expected_realm) = 0
        OR OCTET_LENGTH(expected_identifier) = 0
        OR source_key_count > 1
        OR exact_mapping_count <> 1',
    'SELECT 0 INTO @iam_authn_account_blockers'
);
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

-- Reachable password, phone and OAuth facts must already have canonical
-- counterparts. Password/OAuth rows whose account is absent were unreachable
-- to the old Account JOIN as well; their historical recovery boundary is the
-- verified full backup required immediately before this destructive release.
SET @iam_authn_credential_blockers = 0;
SET @iam_sql = IF(
    @iam_authn_legacy_table_count = 2,
    'WITH credential_kinds AS (
       SELECT lc.*,
         CASE
           WHEN lc.type = ''password''
             AND (COALESCE(OCTET_LENGTH(lc.material), 0) = 0 OR COALESCE(lc.algo, '''') = '''')
             THEN ''invalid_password''
           WHEN (lc.type = ''password'' OR COALESCE(lc.idp, '''') = '''')
             AND COALESCE(OCTET_LENGTH(lc.material), 0) > 0
             AND COALESCE(lc.algo, '''') <> ''''
             THEN ''password''
           WHEN lc.type = ''phone_otp'' OR COALESCE(lc.idp, '''') = ''phone''
             THEN ''phone''
           WHEN lc.type IN (''oauth_wx_minip'', ''oauth_wx_open'', ''oauth_wx_scan'', ''oauth_wecom'')
             THEN ''oauth''
           ELSE ''unknown''
         END AS credential_kind,
         CASE lc.type WHEN ''oauth_wx_minip'' THEN ''wechat_minip'' WHEN ''oauth_wecom'' THEN ''wecom'' ELSE ''wechat_open'' END AS oauth_provider
       FROM auth_credentials_legacy lc
     ), oauth_global_authority AS (
       SELECT provider, global_identifier
       FROM auth_login_identities
       WHERE global_identifier IS NOT NULL AND TRIM(global_identifier) <> ''''
       GROUP BY provider, global_identifier
       HAVING COUNT(*) = 1
     ), blocker_counts AS (
       SELECT COUNT(*) AS blockers
       FROM credential_kinds
       WHERE credential_kind IN (''invalid_password'', ''unknown'')

       UNION ALL

       SELECT COUNT(*)
       FROM credential_kinds ck
       JOIN auth_accounts a ON a.id = ck.account_id
       WHERE ck.credential_kind = ''password''
         AND NOT EXISTS (
           SELECT 1
           FROM auth_login_identities li
           JOIN auth_credentials c ON c.login_identity_id = li.id
           WHERE CAST(c.type AS BINARY) = CAST(''password'' AS BINARY)
             AND li.user_id = a.user_id
             AND CAST(li.provider AS BINARY) = CAST(CASE a.type WHEN ''wc-minip'' THEN ''wechat_minip'' WHEN ''wc-com'' THEN ''wecom'' ELSE ''username'' END AS BINARY)
             AND CAST(li.realm AS BINARY) = CAST(CASE WHEN a.type = ''opera'' AND a.scoped_tenant_id <> 0 THEN CAST(a.scoped_tenant_id AS CHAR)
                                       WHEN a.type IN (''wc-minip'', ''wc-com'') THEN TRIM(a.app_id) ELSE ''default'' END AS BINARY)
             AND CAST(li.identifier AS BINARY) = CAST(TRIM(a.external_id) AS BINARY)
         )

       UNION ALL

       SELECT COUNT(*)
       FROM (
         SELECT account_id
         FROM credential_kinds
         WHERE credential_kind = ''password''
           AND EXISTS (SELECT 1 FROM auth_accounts a WHERE a.id = credential_kinds.account_id)
         GROUP BY account_id
         HAVING COUNT(*) > 1
       ) duplicate_password_sources

       UNION ALL

       SELECT COUNT(*)
       FROM credential_kinds ck
       LEFT JOIN auth_accounts a ON a.id = ck.account_id
       WHERE ck.credential_kind = ''phone''
         AND (a.id IS NULL
           OR OCTET_LENGTH(TRIM(COALESCE(ck.idp_identifier, ''''))) = 0
           OR NOT EXISTS (
             SELECT 1
             FROM auth_login_identities li
             WHERE li.user_id = a.user_id
               AND CAST(li.provider AS BINARY) = CAST(''phone'' AS BINARY)
               AND CAST(li.realm AS BINARY) = CAST(''global'' AS BINARY)
               AND CAST(li.identifier AS BINARY) = CAST(TRIM(COALESCE(ck.idp_identifier, '''')) AS BINARY)
           ))

       UNION ALL

       SELECT COUNT(*)
       FROM (
         SELECT MIN(TRIM(COALESCE(idp_identifier, ''''))) AS phone_identifier
         FROM credential_kinds
         WHERE credential_kind = ''phone''
         GROUP BY CAST(TRIM(COALESCE(idp_identifier, '''')) AS BINARY)
         HAVING COUNT(*) > 1
       ) duplicate_phone_sources

       UNION ALL

       SELECT COUNT(*)
       FROM credential_kinds ck
       JOIN auth_accounts a ON a.id = ck.account_id
       LEFT JOIN auth_login_identities direct_identity
         ON CAST(direct_identity.provider AS BINARY) = CAST(ck.oauth_provider AS BINARY)
        AND CAST(direct_identity.realm AS BINARY) = CAST(TRIM(COALESCE(ck.app_id, '''')) AS BINARY)
        AND CAST(direct_identity.identifier AS BINARY) = CAST(TRIM(COALESCE(ck.idp_identifier, '''')) AS BINARY)
       LEFT JOIN oauth_global_authority global_identity
         ON CAST(global_identity.provider AS BINARY) = CAST(ck.oauth_provider AS BINARY)
        AND CAST(global_identity.global_identifier AS BINARY) = CAST(TRIM(COALESCE(ck.idp_identifier, '''')) AS BINARY)
       WHERE ck.credential_kind = ''oauth''
         AND (CHAR_LENGTH(TRIM(COALESCE(ck.app_id, ''''))) = 0
           OR CHAR_LENGTH(TRIM(COALESCE(ck.idp_identifier, ''''))) = 0
           OR CHAR_LENGTH(TRIM(COALESCE(ck.app_id, ''''))) > 128
           OR CHAR_LENGTH(TRIM(COALESCE(ck.idp_identifier, ''''))) > 255
           OR (direct_identity.id IS NULL AND global_identity.global_identifier IS NULL))
     )
     SELECT COALESCE(SUM(blockers), 0) INTO @iam_authn_credential_blockers
     FROM blocker_counts',
    'SELECT 0 INTO @iam_authn_credential_blockers'
);
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

DROP TEMPORARY TABLE IF EXISTS iam_authn_data_assertion;
CREATE TEMPORARY TABLE iam_authn_data_assertion
(
    message VARCHAR(128) NOT NULL PRIMARY KEY
);
INSERT INTO iam_authn_data_assertion (message)
VALUES ('legacy AuthN reconciliation is incomplete');
INSERT INTO iam_authn_data_assertion (message)
SELECT 'legacy AuthN reconciliation is incomplete'
WHERE @iam_authn_account_blockers <> 0
   OR @iam_authn_credential_blockers <> 0;
DROP TEMPORARY TABLE iam_authn_data_assertion;

-- Fail closed for visible and opaque database-owned dependencies. The only
-- allowed removal is the final atomic two-table DROP below.
SET @iam_authn_retirement_pattern =
    '(^|[^a-z0-9_])(auth_accounts|auth_credentials_legacy)([^a-z0-9_]|$)';
SET @iam_authn_retirement_dependencies = (
      SELECT COUNT(*)
      FROM information_schema.KEY_COLUMN_USAGE
      WHERE (
          TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME IN ('auth_accounts', 'auth_credentials_legacy')
          AND REFERENCED_TABLE_NAME IS NOT NULL
      )
      OR (
          REFERENCED_TABLE_SCHEMA = DATABASE()
          AND REFERENCED_TABLE_NAME IN ('auth_accounts', 'auth_credentials_legacy')
      )
)
+ (
      SELECT COUNT(*)
      FROM information_schema.TRIGGERS
      WHERE TRIGGER_SCHEMA = DATABASE()
        AND (
            EVENT_OBJECT_TABLE IN ('auth_accounts', 'auth_credentials_legacy')
            OR LOWER(ACTION_STATEMENT) REGEXP @iam_authn_retirement_pattern
        )
)
+ (
      SELECT COUNT(*)
      FROM information_schema.VIEWS
      WHERE TABLE_SCHEMA = DATABASE()
        AND (
            VIEW_DEFINITION IS NULL
            OR LOWER(VIEW_DEFINITION) REGEXP @iam_authn_retirement_pattern
        )
)
+ (
      SELECT COUNT(*)
      FROM information_schema.ROUTINES
      WHERE ROUTINE_SCHEMA = DATABASE()
        AND (
            ROUTINE_DEFINITION IS NULL
            OR LOWER(ROUTINE_DEFINITION) REGEXP @iam_authn_retirement_pattern
        )
)
+ (
      SELECT COUNT(*)
      FROM information_schema.EVENTS
      WHERE EVENT_SCHEMA = DATABASE()
        AND (
            EVENT_DEFINITION IS NULL
            OR LOWER(EVENT_DEFINITION) REGEXP @iam_authn_retirement_pattern
        )
);

DROP TEMPORARY TABLE IF EXISTS iam_authn_dependency_assertion;
CREATE TEMPORARY TABLE iam_authn_dependency_assertion
(
    message VARCHAR(128) NOT NULL PRIMARY KEY
);
INSERT INTO iam_authn_dependency_assertion (message)
VALUES ('legacy AuthN database dependencies still exist');
INSERT INTO iam_authn_dependency_assertion (message)
SELECT 'legacy AuthN database dependencies still exist'
WHERE @iam_authn_retirement_dependencies <> 0;
DROP TEMPORARY TABLE iam_authn_dependency_assertion;

DROP TABLE IF EXISTS auth_credentials_legacy, auth_accounts;
