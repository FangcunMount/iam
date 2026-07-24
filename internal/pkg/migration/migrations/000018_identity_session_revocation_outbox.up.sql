CREATE TABLE IF NOT EXISTS `identity_session_revocation_outbox` (
    `task_id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id` BIGINT UNSIGNED NOT NULL,
    `user_version` INT UNSIGNED NOT NULL,
    `action` VARCHAR(32) NOT NULL,
    `reason` VARCHAR(64) NOT NULL,
    `status` VARCHAR(16) NOT NULL DEFAULT 'pending',
    `attempt_count` INT UNSIGNED NOT NULL DEFAULT 0,
    `next_attempt_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    `last_error` VARCHAR(255) NULL,
    `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    `completed_at` DATETIME(3) NULL,
    PRIMARY KEY (`task_id`),
    UNIQUE KEY `uk_identity_session_revocation_version_action` (`user_id`, `user_version`, `action`),
    KEY `idx_identity_session_revocation_claim` (`status`, `next_attempt_at`),
    KEY `idx_identity_session_revocation_user` (`user_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
