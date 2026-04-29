-- ============================================================================
-- Migration: Add profile_id index to profile_links table
-- Version: 000002
-- Description: Add index on profile_id column for faster profile-based lookups
-- Date: 2025-12-08
-- ============================================================================

-- 为 profile_links 表的 profile_id 列添加索引
-- 用于优化通过档案 ID 查询档案关系的性能
CREATE INDEX `idx_profile_id` ON `profile_links` (`profile_id`);
