-- Retire four isolated cleanup/backup tables left by the 2026-08-12 seeddata
-- duplicate cleanup. Production evidence established that both profile copies
-- and both profile-link copies contain the same 1,359 rows, none of their IDs
-- remain in the canonical tables, and no database-owned dependency references
-- them. This migration never writes canonical data.

SET @iam_cleanup_table_count = (
    SELECT COUNT(*)
    FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_TYPE = 'BASE TABLE'
      AND TABLE_NAME IN (
        'cbpt_profiles_s812v2',
        'cbpt_profile_links_s812v2',
        'cleanup_bak_perf_testee_profiles_seeddata_dup_20260812_v1',
        'cleanup_bak_perf_testee_profile_links_seeddata_dup_20260812_v1'
      )
);
SET @iam_cleanup_canonical_shape = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND (TABLE_NAME = 'profiles' AND COLUMN_NAME = 'id'
        OR TABLE_NAME = 'profile_links' AND COLUMN_NAME = 'id')
);
SET @iam_cleanup_canonical_table_count = (
    SELECT COUNT(*)
    FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_TYPE = 'BASE TABLE'
      AND TABLE_NAME IN ('profiles', 'profile_links')
);
SET @iam_cleanup_profile_shapes = (
    SELECT COUNT(*)
    FROM (
      SELECT TABLE_NAME
      FROM information_schema.COLUMNS
      WHERE TABLE_SCHEMA = DATABASE()
        AND TABLE_NAME IN (
          'cbpt_profiles_s812v2',
          'cleanup_bak_perf_testee_profiles_seeddata_dup_20260812_v1'
        )
      GROUP BY TABLE_NAME
      HAVING GROUP_CONCAT(
        CONCAT(ORDINAL_POSITION, ':', COLUMN_NAME, ':', COLUMN_TYPE)
        ORDER BY ORDINAL_POSITION SEPARATOR ','
      ) = '1:id:bigint unsigned,2:name:varchar(64),3:id_card:varchar(20),4:gender:tinyint,5:birthday:varchar(10),6:created_at:datetime,7:updated_at:datetime,8:deleted_at:datetime,9:created_by:bigint unsigned,10:updated_by:bigint unsigned,11:deleted_by:bigint unsigned,12:version:int unsigned'
    ) profile_shapes
);
SET @iam_cleanup_profile_link_shapes = (
    SELECT COUNT(*)
    FROM (
      SELECT TABLE_NAME
      FROM information_schema.COLUMNS
      WHERE TABLE_SCHEMA = DATABASE()
        AND TABLE_NAME IN (
          'cbpt_profile_links_s812v2',
          'cleanup_bak_perf_testee_profile_links_seeddata_dup_20260812_v1'
        )
      GROUP BY TABLE_NAME
      HAVING GROUP_CONCAT(
        CONCAT(ORDINAL_POSITION, ':', COLUMN_NAME, ':', COLUMN_TYPE)
        ORDER BY ORDINAL_POSITION SEPARATOR ','
      ) = '1:id:bigint unsigned,2:user_id:bigint unsigned,3:profile_id:bigint unsigned,4:type:varchar(32),5:relation:varchar(16),6:self_key:bigint unsigned,7:established_at:datetime,8:revoked_at:datetime,9:created_at:datetime,10:updated_at:datetime,11:deleted_at:datetime,12:created_by:bigint unsigned,13:updated_by:bigint unsigned,14:deleted_by:bigint unsigned,15:version:int unsigned'
    ) profile_link_shapes
);

DROP TEMPORARY TABLE IF EXISTS iam_cleanup_schema_assertion;
CREATE TEMPORARY TABLE iam_cleanup_schema_assertion
(
    message VARCHAR(128) NOT NULL PRIMARY KEY
);
INSERT INTO iam_cleanup_schema_assertion (message)
VALUES ('seeddata cleanup table set or schema differs from verified evidence');
INSERT INTO iam_cleanup_schema_assertion (message)
SELECT 'seeddata cleanup table set or schema differs from verified evidence'
WHERE @iam_cleanup_table_count NOT IN (0, 4)
   OR @iam_cleanup_canonical_table_count <> 2
   OR @iam_cleanup_canonical_shape <> 2
   OR (@iam_cleanup_table_count = 4
     AND (@iam_cleanup_profile_shapes <> 2 OR @iam_cleanup_profile_link_shapes <> 2));
DROP TEMPORARY TABLE iam_cleanup_schema_assertion;

-- Referencing the cleanup tables must be conditional so a fresh database, on
-- which these one-off objects never existed, can still apply the migration.
SET @iam_cleanup_data_blockers = 0;
SET @iam_sql = IF(
  @iam_cleanup_table_count = 4,
  'SELECT COUNT(*) INTO @iam_cleanup_data_blockers
   FROM (
     SELECT 1 AS blocker
     WHERE (SELECT COUNT(*) FROM cbpt_profiles_s812v2) <> 1359
        OR (SELECT COUNT(*) FROM cleanup_bak_perf_testee_profiles_seeddata_dup_20260812_v1) <> 1359
        OR (SELECT COUNT(*) FROM cbpt_profile_links_s812v2) <> 1359
        OR (SELECT COUNT(*) FROM cleanup_bak_perf_testee_profile_links_seeddata_dup_20260812_v1) <> 1359

     UNION ALL SELECT 1 WHERE EXISTS (
       SELECT 1
       FROM cbpt_profiles_s812v2 a
       LEFT JOIN cleanup_bak_perf_testee_profiles_seeddata_dup_20260812_v1 b ON b.id = a.id
       WHERE b.id IS NULL OR NOT (
         BINARY a.name <=> BINARY b.name AND BINARY a.id_card <=> BINARY b.id_card
         AND a.gender <=> b.gender AND BINARY a.birthday <=> BINARY b.birthday
         AND a.created_at <=> b.created_at
         AND a.updated_at <=> b.updated_at AND a.deleted_at <=> b.deleted_at
         AND a.created_by <=> b.created_by AND a.updated_by <=> b.updated_by
         AND a.deleted_by <=> b.deleted_by AND a.version <=> b.version
       )
     )
     UNION ALL SELECT 1 WHERE EXISTS (
       SELECT 1
       FROM cleanup_bak_perf_testee_profiles_seeddata_dup_20260812_v1 a
       LEFT JOIN cbpt_profiles_s812v2 b ON b.id = a.id
       WHERE b.id IS NULL OR NOT (
         BINARY a.name <=> BINARY b.name AND BINARY a.id_card <=> BINARY b.id_card
         AND a.gender <=> b.gender AND BINARY a.birthday <=> BINARY b.birthday
         AND a.created_at <=> b.created_at
         AND a.updated_at <=> b.updated_at AND a.deleted_at <=> b.deleted_at
         AND a.created_by <=> b.created_by AND a.updated_by <=> b.updated_by
         AND a.deleted_by <=> b.deleted_by AND a.version <=> b.version
       )
     )
     UNION ALL SELECT 1 WHERE EXISTS (
       SELECT 1
       FROM cbpt_profile_links_s812v2 a
       LEFT JOIN cleanup_bak_perf_testee_profile_links_seeddata_dup_20260812_v1 b ON b.id = a.id
       WHERE b.id IS NULL OR NOT (
         a.user_id <=> b.user_id AND a.profile_id <=> b.profile_id
         AND BINARY a.type <=> BINARY b.type AND BINARY a.relation <=> BINARY b.relation
         AND a.self_key <=> b.self_key
         AND a.established_at <=> b.established_at AND a.revoked_at <=> b.revoked_at
         AND a.created_at <=> b.created_at AND a.updated_at <=> b.updated_at
         AND a.deleted_at <=> b.deleted_at AND a.created_by <=> b.created_by
         AND a.updated_by <=> b.updated_by AND a.deleted_by <=> b.deleted_by
         AND a.version <=> b.version
       )
     )
     UNION ALL SELECT 1 WHERE EXISTS (
       SELECT 1
       FROM cleanup_bak_perf_testee_profile_links_seeddata_dup_20260812_v1 a
       LEFT JOIN cbpt_profile_links_s812v2 b ON b.id = a.id
       WHERE b.id IS NULL OR NOT (
         a.user_id <=> b.user_id AND a.profile_id <=> b.profile_id
         AND BINARY a.type <=> BINARY b.type AND BINARY a.relation <=> BINARY b.relation
         AND a.self_key <=> b.self_key
         AND a.established_at <=> b.established_at AND a.revoked_at <=> b.revoked_at
         AND a.created_at <=> b.created_at AND a.updated_at <=> b.updated_at
         AND a.deleted_at <=> b.deleted_at AND a.created_by <=> b.created_by
         AND a.updated_by <=> b.updated_by AND a.deleted_by <=> b.deleted_by
         AND a.version <=> b.version
       )
     )
     UNION ALL SELECT 1 WHERE EXISTS (
       SELECT 1 FROM cbpt_profiles_s812v2 backup JOIN profiles canonical ON canonical.id = backup.id
     )
     UNION ALL SELECT 1 WHERE EXISTS (
       SELECT 1 FROM cleanup_bak_perf_testee_profiles_seeddata_dup_20260812_v1 backup JOIN profiles canonical ON canonical.id = backup.id
     )
     UNION ALL SELECT 1 WHERE EXISTS (
       SELECT 1 FROM cbpt_profile_links_s812v2 backup JOIN profile_links canonical ON canonical.id = backup.id
     )
     UNION ALL SELECT 1 WHERE EXISTS (
       SELECT 1 FROM cleanup_bak_perf_testee_profile_links_seeddata_dup_20260812_v1 backup JOIN profile_links canonical ON canonical.id = backup.id
     )
   ) blockers',
  'SELECT 0 INTO @iam_cleanup_data_blockers'
);
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

DROP TEMPORARY TABLE IF EXISTS iam_cleanup_data_assertion;
CREATE TEMPORARY TABLE iam_cleanup_data_assertion
(
    message VARCHAR(128) NOT NULL PRIMARY KEY
);
INSERT INTO iam_cleanup_data_assertion (message)
VALUES ('seeddata cleanup table contents differ from verified evidence');
INSERT INTO iam_cleanup_data_assertion (message)
SELECT 'seeddata cleanup table contents differ from verified evidence'
WHERE @iam_cleanup_data_blockers <> 0;
DROP TEMPORARY TABLE iam_cleanup_data_assertion;

SET @iam_cleanup_retirement_pattern =
  '(^|[^a-z0-9_])(cbpt_profiles_s812v2|cbpt_profile_links_s812v2|cleanup_bak_perf_testee_profiles_seeddata_dup_20260812_v1|cleanup_bak_perf_testee_profile_links_seeddata_dup_20260812_v1)([^a-z0-9_]|$)';
SET @iam_cleanup_dependencies = (
    SELECT COUNT(*)
    FROM information_schema.KEY_COLUMN_USAGE
    WHERE (TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME IN ('cbpt_profiles_s812v2', 'cbpt_profile_links_s812v2', 'cleanup_bak_perf_testee_profiles_seeddata_dup_20260812_v1', 'cleanup_bak_perf_testee_profile_links_seeddata_dup_20260812_v1')
      AND REFERENCED_TABLE_NAME IS NOT NULL)
       OR (REFERENCED_TABLE_SCHEMA = DATABASE()
      AND REFERENCED_TABLE_NAME IN ('cbpt_profiles_s812v2', 'cbpt_profile_links_s812v2', 'cleanup_bak_perf_testee_profiles_seeddata_dup_20260812_v1', 'cleanup_bak_perf_testee_profile_links_seeddata_dup_20260812_v1'))
  ) + (
    SELECT COUNT(*)
    FROM information_schema.TRIGGERS
    WHERE TRIGGER_SCHEMA = DATABASE()
      AND (EVENT_OBJECT_TABLE IN ('cbpt_profiles_s812v2', 'cbpt_profile_links_s812v2', 'cleanup_bak_perf_testee_profiles_seeddata_dup_20260812_v1', 'cleanup_bak_perf_testee_profile_links_seeddata_dup_20260812_v1')
        OR ACTION_STATEMENT IS NULL
        OR LOWER(ACTION_STATEMENT) REGEXP @iam_cleanup_retirement_pattern)
  ) + (
    SELECT COUNT(*)
    FROM information_schema.VIEWS
    WHERE TABLE_SCHEMA = DATABASE()
      AND (VIEW_DEFINITION IS NULL OR LOWER(VIEW_DEFINITION) REGEXP @iam_cleanup_retirement_pattern)
  ) + (
    SELECT COUNT(*)
    FROM information_schema.ROUTINES
    WHERE ROUTINE_SCHEMA = DATABASE()
      AND (ROUTINE_DEFINITION IS NULL OR LOWER(ROUTINE_DEFINITION) REGEXP @iam_cleanup_retirement_pattern)
  ) + (
    SELECT COUNT(*)
    FROM information_schema.EVENTS
    WHERE EVENT_SCHEMA = DATABASE()
      AND (EVENT_DEFINITION IS NULL OR LOWER(EVENT_DEFINITION) REGEXP @iam_cleanup_retirement_pattern)
  ) + (
    SELECT COUNT(*)
    FROM information_schema.TABLE_PRIVILEGES
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME IN ('cbpt_profiles_s812v2', 'cbpt_profile_links_s812v2', 'cleanup_bak_perf_testee_profiles_seeddata_dup_20260812_v1', 'cleanup_bak_perf_testee_profile_links_seeddata_dup_20260812_v1')
  );

DROP TEMPORARY TABLE IF EXISTS iam_cleanup_dependency_assertion;
CREATE TEMPORARY TABLE iam_cleanup_dependency_assertion
(
    message VARCHAR(128) NOT NULL PRIMARY KEY
);
INSERT INTO iam_cleanup_dependency_assertion (message)
VALUES ('seeddata cleanup table database dependencies still exist');
INSERT INTO iam_cleanup_dependency_assertion (message)
SELECT 'seeddata cleanup table database dependencies still exist'
WHERE @iam_cleanup_dependencies <> 0;
DROP TEMPORARY TABLE iam_cleanup_dependency_assertion;

DROP TABLE IF EXISTS
  cbpt_profile_links_s812v2,
  cbpt_profiles_s812v2,
  cleanup_bak_perf_testee_profile_links_seeddata_dup_20260812_v1,
  cleanup_bak_perf_testee_profiles_seeddata_dup_20260812_v1;
