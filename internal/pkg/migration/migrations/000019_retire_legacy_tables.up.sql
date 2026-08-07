-- Retire the legacy Identity tables children and guardianships.
-- Every data/dependency assertion runs before the single final DROP statement.
-- The migration is intentionally fail-closed: incomplete canonical mappings
-- leave both legacy tables in place and require operator remediation.

-- Recreate the canonical Identity schema for databases whose historical
-- schema_migrations version=2 did not execute the current 000002 contents.
CREATE TABLE IF NOT EXISTS profiles
(
    id         BIGINT UNSIGNED NOT NULL PRIMARY KEY,
    name       VARCHAR(64)     NOT NULL,
    id_card    VARCHAR(20)              DEFAULT NULL,
    gender     TINYINT         NOT NULL DEFAULT 0,
    birthday   VARCHAR(10)              DEFAULT NULL,
    height     BIGINT                   DEFAULT NULL,
    weight     BIGINT                   DEFAULT NULL,
    created_at DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at DATETIME                 DEFAULT NULL,
    created_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
    updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
    deleted_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
    version    INT UNSIGNED    NOT NULL DEFAULT 1,
    UNIQUE KEY uk_id_card (id_card),
    KEY idx_deleted_at (deleted_at),
    KEY idx_name_gender_birthday (name, gender, birthday)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS profile_links
(
    id             BIGINT UNSIGNED NOT NULL PRIMARY KEY,
    user_id        BIGINT UNSIGNED NOT NULL,
    profile_id     BIGINT UNSIGNED NOT NULL,
    type           VARCHAR(32)     NOT NULL DEFAULT 'relation',
    relation       VARCHAR(16)     NOT NULL,
    self_key       BIGINT UNSIGNED          DEFAULT NULL,
    established_at DATETIME        NOT NULL,
    revoked_at     DATETIME                 DEFAULT NULL,
    created_at     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at     DATETIME                 DEFAULT NULL,
    created_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
    updated_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
    deleted_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
    version        INT UNSIGNED    NOT NULL DEFAULT 1,
    UNIQUE KEY uk_user_profile_link (user_id, profile_id, type),
    KEY idx_profile_id (profile_id),
    KEY idx_type (type),
    KEY idx_deleted_at (deleted_at)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;

SET @iam_has_self_key = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'profile_links'
      AND COLUMN_NAME = 'self_key'
);
SET @iam_sql = IF(
    @iam_has_self_key = 0,
    'ALTER TABLE profile_links ADD COLUMN self_key BIGINT UNSIGNED DEFAULT NULL AFTER relation',
    'SELECT 1'
);
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

SET @iam_has_children = (
    SELECT COUNT(*)
    FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'children'
);
SET @iam_sql = IF(
    @iam_has_children > 0,
    'INSERT IGNORE INTO profiles
       (id, name, id_card, gender, birthday, height, weight, created_at, updated_at,
        deleted_at, created_by, updated_by, deleted_by, version)
     SELECT id, name, id_card, gender, birthday, height, weight, created_at, updated_at,
            deleted_at, created_by, updated_by, deleted_by, version
     FROM children',
    'SELECT 1'
);
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

SET @iam_has_guardianships = (
    SELECT COUNT(*)
    FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'guardianships'
);
SET @iam_sql = IF(
    @iam_has_guardianships > 0,
    'INSERT IGNORE INTO profile_links
       (id, user_id, profile_id, type, relation, self_key, established_at, revoked_at,
        created_at, updated_at, deleted_at, created_by, updated_by, deleted_by, version)
     SELECT id,
            user_id,
            child_id,
            IF(LOWER(TRIM(relation)) = ''self'', ''self'', ''relation''),
            CASE LOWER(TRIM(relation))
                WHEN ''self'' THEN ''self''
                WHEN ''parent'' THEN ''parent''
                WHEN ''grandparent'' THEN ''grandparent''
                ELSE ''other''
            END,
            CASE
                WHEN LOWER(TRIM(relation)) = ''self'' AND revoked_at IS NULL AND deleted_at IS NULL THEN user_id
                ELSE NULL
            END,
            COALESCE(established_at, created_at, NOW()),
            revoked_at,
            COALESCE(created_at, NOW()),
            COALESCE(updated_at, created_at, NOW()),
            deleted_at,
            created_by,
            updated_by,
            deleted_by,
            version
     FROM guardianships',
    'SELECT 1'
);
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

UPDATE profile_links
SET self_key = CASE
    WHEN type = 'self' AND revoked_at IS NULL AND deleted_at IS NULL THEN user_id
    ELSE NULL
END;

SET @iam_has_profile_id_index = (
    SELECT COUNT(*)
    FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'profile_links'
      AND INDEX_NAME = 'idx_profile_id'
);
SET @iam_sql = IF(
    @iam_has_profile_id_index = 0,
    'CREATE INDEX idx_profile_id ON profile_links (profile_id)',
    'SELECT 1'
);
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

SET @iam_has_active_self_guard = (
    SELECT COUNT(*)
    FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'profile_links'
      AND INDEX_NAME = 'uk_active_self_profile_link'
);
SET @iam_sql = IF(
    @iam_has_active_self_guard = 0,
    'CREATE UNIQUE INDEX uk_active_self_profile_link ON profile_links (self_key)',
    'SELECT 1'
);
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

-- Identity parity must be complete before legacy facts are removed.
SET @iam_children_mismatches = 0;
SET @iam_sql = IF(
    @iam_has_children > 0,
    'SELECT COUNT(*) INTO @iam_children_mismatches
     FROM children c
     LEFT JOIN profiles p ON p.id = c.id
     WHERE p.id IS NULL
        OR NOT (
          p.name <=> c.name
          AND p.id_card <=> c.id_card
          AND p.gender <=> c.gender
          AND p.birthday <=> c.birthday
          AND p.height <=> c.height
          AND p.weight <=> c.weight
          AND p.created_at <=> c.created_at
          AND p.updated_at <=> c.updated_at
          AND p.deleted_at <=> c.deleted_at
          AND p.created_by <=> c.created_by
          AND p.updated_by <=> c.updated_by
          AND p.deleted_by <=> c.deleted_by
          AND p.version <=> c.version
        )',
    'SELECT 0 INTO @iam_children_mismatches'
);
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

SET @iam_guardianship_mismatches = 0;
SET @iam_sql = IF(
    @iam_has_guardianships > 0,
    'SELECT COUNT(*) INTO @iam_guardianship_mismatches
     FROM guardianships g
     LEFT JOIN profile_links p ON p.id = g.id
     WHERE p.id IS NULL
        OR NOT (
          p.user_id <=> g.user_id
          AND p.profile_id <=> g.child_id
          AND p.type <=> IF(LOWER(TRIM(g.relation)) = ''self'', ''self'', ''relation'')
          AND p.relation <=> CASE LOWER(TRIM(g.relation))
              WHEN ''self'' THEN ''self''
              WHEN ''parent'' THEN ''parent''
              WHEN ''grandparent'' THEN ''grandparent''
              ELSE ''other''
          END
          AND p.self_key <=> CASE
              WHEN LOWER(TRIM(g.relation)) = ''self'' AND g.revoked_at IS NULL AND g.deleted_at IS NULL THEN g.user_id
              ELSE NULL
          END
          AND p.established_at <=> COALESCE(g.established_at, g.created_at)
          AND p.revoked_at <=> g.revoked_at
          AND p.created_at <=> g.created_at
          AND p.updated_at <=> g.updated_at
          AND p.deleted_at <=> g.deleted_at
          AND p.created_by <=> g.created_by
          AND p.updated_by <=> g.updated_by
          AND p.deleted_by <=> g.deleted_by
          AND p.version <=> g.version
        )',
    'SELECT 0 INTO @iam_guardianship_mismatches'
);
PREPARE iam_stmt FROM @iam_sql;
EXECUTE iam_stmt;
DEALLOCATE PREPARE iam_stmt;

SET @iam_identity_mismatches =
    @iam_children_mismatches + @iam_guardianship_mismatches;
-- MySQL 8 cannot execute SIGNAL through PREPARE. An intentional duplicate key
-- keeps the conditional assertion session-local and includes the reason in the error.
DROP TEMPORARY TABLE IF EXISTS iam_identity_retirement_assertion;
CREATE TEMPORARY TABLE iam_identity_retirement_assertion
(
    message VARCHAR(128) NOT NULL PRIMARY KEY
);
INSERT INTO iam_identity_retirement_assertion (message)
VALUES ('legacy Identity parity is incomplete');
INSERT INTO iam_identity_retirement_assertion (message)
SELECT 'legacy Identity parity is incomplete'
WHERE @iam_identity_mismatches <> 0;
DROP TEMPORARY TABLE iam_identity_retirement_assertion;

-- A DROP must not silently remove database-owned dependencies. Opaque
-- definitions also fail closed because their references cannot be ruled out.
SET @iam_retirement_pattern =
    '(^|[^a-z0-9_])(children|guardianships)([^a-z0-9_]|$)';
SET @iam_retirement_dependencies = (
      SELECT COUNT(*)
      FROM information_schema.KEY_COLUMN_USAGE
      WHERE (
          TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME IN ('children', 'guardianships')
          AND REFERENCED_TABLE_NAME IS NOT NULL
      )
      OR (
          REFERENCED_TABLE_SCHEMA = DATABASE()
          AND REFERENCED_TABLE_NAME IN ('children', 'guardianships')
      )
)
+ (
      SELECT COUNT(*)
      FROM information_schema.TRIGGERS
      WHERE TRIGGER_SCHEMA = DATABASE()
        AND (
            EVENT_OBJECT_TABLE IN ('children', 'guardianships')
            OR LOWER(ACTION_STATEMENT) REGEXP @iam_retirement_pattern
        )
)
+ (
      SELECT COUNT(*)
      FROM information_schema.VIEWS
      WHERE TABLE_SCHEMA = DATABASE()
        AND (VIEW_DEFINITION IS NULL OR LOWER(VIEW_DEFINITION) REGEXP @iam_retirement_pattern)
)
+ (
      SELECT COUNT(*)
      FROM information_schema.ROUTINES
      WHERE ROUTINE_SCHEMA = DATABASE()
        AND (ROUTINE_DEFINITION IS NULL OR LOWER(ROUTINE_DEFINITION) REGEXP @iam_retirement_pattern)
)
+ (
      SELECT COUNT(*)
      FROM information_schema.EVENTS
      WHERE EVENT_SCHEMA = DATABASE()
        AND (EVENT_DEFINITION IS NULL OR LOWER(EVENT_DEFINITION) REGEXP @iam_retirement_pattern)
);

-- Use the same fail-closed assertion for database-owned dependencies.
DROP TEMPORARY TABLE IF EXISTS iam_identity_dependency_assertion;
CREATE TEMPORARY TABLE iam_identity_dependency_assertion
(
    message VARCHAR(128) NOT NULL PRIMARY KEY
);
INSERT INTO iam_identity_dependency_assertion (message)
VALUES ('legacy table database dependencies still exist');
INSERT INTO iam_identity_dependency_assertion (message)
SELECT 'legacy table database dependencies still exist'
WHERE @iam_retirement_dependencies <> 0;
DROP TEMPORARY TABLE iam_identity_dependency_assertion;

-- MySQL 8 atomic DDL keeps the destructive step as one final statement.
DROP TABLE IF EXISTS
    children,
    guardianships;
