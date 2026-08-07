-- Retire the redundant schema_version table. golang-migrate owns the canonical
-- schema state in schema_migrations; database-owned dependencies still fail
-- closed before the final irreversible removal.

SET @iam_retirement_pattern =
    '(^|[^a-z0-9_])(schema_version)([^a-z0-9_]|$)';
SET @iam_retirement_dependencies = (
      SELECT COUNT(*)
      FROM information_schema.KEY_COLUMN_USAGE
      WHERE (
          TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'schema_version'
          AND REFERENCED_TABLE_NAME IS NOT NULL
      )
      OR (
          REFERENCED_TABLE_SCHEMA = DATABASE()
          AND REFERENCED_TABLE_NAME = 'schema_version'
      )
)
+ (
      SELECT COUNT(*)
      FROM information_schema.TRIGGERS
      WHERE TRIGGER_SCHEMA = DATABASE()
        AND (
            EVENT_OBJECT_TABLE = 'schema_version'
            OR LOWER(ACTION_STATEMENT) REGEXP @iam_retirement_pattern
        )
)
+ (
      SELECT COUNT(*)
      FROM information_schema.VIEWS
      WHERE TABLE_SCHEMA = DATABASE()
        AND (
            VIEW_DEFINITION IS NULL
            OR LOWER(VIEW_DEFINITION) REGEXP @iam_retirement_pattern
        )
)
+ (
      SELECT COUNT(*)
      FROM information_schema.ROUTINES
      WHERE ROUTINE_SCHEMA = DATABASE()
        AND (
            ROUTINE_DEFINITION IS NULL
            OR LOWER(ROUTINE_DEFINITION) REGEXP @iam_retirement_pattern
        )
)
+ (
      SELECT COUNT(*)
      FROM information_schema.EVENTS
      WHERE EVENT_SCHEMA = DATABASE()
        AND (
            EVENT_DEFINITION IS NULL
            OR LOWER(EVENT_DEFINITION) REGEXP @iam_retirement_pattern
        )
);

DROP TEMPORARY TABLE IF EXISTS iam_schema_version_retirement_assertion;
CREATE TEMPORARY TABLE iam_schema_version_retirement_assertion
(
    message VARCHAR(128) NOT NULL PRIMARY KEY
);
INSERT INTO iam_schema_version_retirement_assertion (message)
VALUES ('schema_version database dependencies still exist');
INSERT INTO iam_schema_version_retirement_assertion (message)
SELECT 'schema_version database dependencies still exist'
WHERE @iam_retirement_dependencies <> 0;
DROP TEMPORARY TABLE iam_schema_version_retirement_assertion;

DROP TABLE IF EXISTS schema_version;
