ALTER TABLE `jwks_keys`
    DROP INDEX `uk_jwks_keys_single_active`,
    DROP COLUMN `active_guard`;
