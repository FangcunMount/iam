-- Retire unused MySQL log/audit tables. Runtime auditing remains in structured
-- application logs; no repository, API, event, or token path writes these
-- physical tables. Database-owned dependencies still fail closed before the
-- single final removal statement.

SET @iam_retirement_pattern =
    '(^|[^a-z0-9_])(operation_logs|audit_logs|auth_token_audit)([^a-z0-9_]|$)';
SET @iam_retirement_dependencies = (
      SELECT COUNT(*)
      FROM information_schema.KEY_COLUMN_USAGE
      WHERE (
          TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME IN ('operation_logs', 'audit_logs', 'auth_token_audit')
          AND REFERENCED_TABLE_NAME IS NOT NULL
      )
      OR (
          REFERENCED_TABLE_SCHEMA = DATABASE()
          AND REFERENCED_TABLE_NAME IN ('operation_logs', 'audit_logs', 'auth_token_audit')
      )
)
+ (
      SELECT COUNT(*)
      FROM information_schema.TRIGGERS
      WHERE TRIGGER_SCHEMA = DATABASE()
        AND (
            EVENT_OBJECT_TABLE IN ('operation_logs', 'audit_logs', 'auth_token_audit')
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

DROP TEMPORARY TABLE IF EXISTS iam_audit_retirement_assertion;
CREATE TEMPORARY TABLE iam_audit_retirement_assertion
(
    message VARCHAR(128) NOT NULL PRIMARY KEY
);
INSERT INTO iam_audit_retirement_assertion (message)
VALUES ('audit table database dependencies still exist');
INSERT INTO iam_audit_retirement_assertion (message)
SELECT 'audit table database dependencies still exist'
WHERE @iam_retirement_dependencies <> 0;
DROP TEMPORARY TABLE iam_audit_retirement_assertion;

DROP TABLE IF EXISTS operation_logs, audit_logs, auth_token_audit;
