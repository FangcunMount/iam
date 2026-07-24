-- Enforce the invariant that at most one JWKS signing key is active.
-- Creating the unique index intentionally fails when legacy data contains
-- multiple active rows; operators must reconcile those rows explicitly.
ALTER TABLE `jwks_keys`
    ADD COLUMN `active_guard` TINYINT
        GENERATED ALWAYS AS (
            CASE WHEN `status` = 1 THEN 1 ELSE NULL END
        ) STORED,
    ADD UNIQUE INDEX `uk_jwks_keys_single_active` (`active_guard`);
