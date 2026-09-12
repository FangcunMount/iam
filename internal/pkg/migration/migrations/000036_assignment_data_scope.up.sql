-- Scope is granted on a role assignment within a company. Existing assignments
-- remain unconfigured; this migration does not grant all-store access.
ALTER TABLE authz_assignments
    ADD COLUMN org_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '范围所属公司；0为未配置' AFTER role_id,
    ADD COLUMN scope_kind VARCHAR(32) NOT NULL DEFAULT '' AFTER org_id,
    ADD COLUMN scope_store_ids JSON NULL AFTER scope_kind,
    DROP INDEX uk_authz_assignments_active,
    ADD UNIQUE INDEX uk_authz_assignments_active(subject_type,subject_id,role_id,org_id,active_guard);
