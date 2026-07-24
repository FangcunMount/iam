ALTER TABLE `users`
    ADD COLUMN `active_phone` VARCHAR(20)
        GENERATED ALWAYS AS (
            CASE
                WHEN `deleted_at` IS NULL AND `phone` IS NOT NULL AND `phone` <> '' THEN `phone`
                ELSE NULL
            END
        ) STORED,
    ADD UNIQUE INDEX `uk_users_active_phone` (`active_phone`);
