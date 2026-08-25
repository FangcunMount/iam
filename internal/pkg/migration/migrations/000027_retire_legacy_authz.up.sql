-- This is a forward-only retirement migration. A populated legacy policy
-- table requires evidence written by `iam-maintenance authz-cutover apply`.
-- A genuinely fresh database with no legacy policy facts may proceed without
-- a marker.
SET @iam_legacy_authz_fact_count = (
    SELECT COUNT(*) FROM `casbin_rule`
);
SET @iam_verified_cutover_count = (
    SELECT COUNT(*)
    FROM `authz_cutover_state`
    WHERE `id` = 1
      AND `status` = 'verified'
      AND `evidence_hash` REGEXP '^[0-9a-f]{64}$'
      AND `verified_at` IS NOT NULL
);
SET @iam_cutover_gate_sql = IF(
    @iam_legacy_authz_fact_count = 0 OR @iam_verified_cutover_count = 1,
    'SELECT 1',
    'SELECT iam_authz_cutover_verification_is_required()'
);
PREPARE iam_cutover_gate_stmt FROM @iam_cutover_gate_sql;
EXECUTE iam_cutover_gate_stmt;
DEALLOCATE PREPARE iam_cutover_gate_stmt;

DROP TABLE `casbin_rule`;
ALTER TABLE `authz_resources` DROP COLUMN `scope_kinds`;
DROP TABLE `authz_cutover_state`;
