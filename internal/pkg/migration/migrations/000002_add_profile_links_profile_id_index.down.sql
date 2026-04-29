-- ============================================================================
-- Migration Rollback: Remove profile_id index from profile_links table
-- Version: 000002
-- Date: 2025-12-08
-- ============================================================================

-- 删除 profile_links 表的 profile_id 索引
DROP INDEX `idx_profile_id` ON `profile_links`;
