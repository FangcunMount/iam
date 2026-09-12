-- Never discard configured scopes or collapse company-specific assignments.
-- Restore data and application contracts together before removing this schema.
SET @iam_scoped_assignment_count = (
    SELECT COUNT(*) FROM authz_assignments
    WHERE org_id <> 0 OR scope_kind <> '' OR scope_store_ids IS NOT NULL
);
SET @iam_scope_down_sql = IF(@iam_scoped_assignment_count = 0,
    'ALTER TABLE authz_assignments DROP INDEX uk_authz_assignments_active, DROP COLUMN scope_store_ids, DROP COLUMN scope_kind, DROP COLUMN org_id, ADD UNIQUE INDEX uk_authz_assignments_active(subject_type,subject_id,role_id,active_guard)',
    'SELECT iam_scope_rollback_requires_coordinated_restore()');
PREPARE iam_scope_down_stmt FROM @iam_scope_down_sql;
EXECUTE iam_scope_down_stmt;
DEALLOCATE PREPARE iam_scope_down_stmt;
