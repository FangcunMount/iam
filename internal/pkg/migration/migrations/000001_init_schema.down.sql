-- ============================================================================
-- IAM - Database Schema Rollback
-- Version: 1.0 (Consolidated)
-- Description: Drop all tables
-- Date: 2025-12-03
-- ============================================================================

-- Drop tables in reverse order of dependencies

-- Schema version
DROP TABLE IF EXISTS `schema_version`;

-- System / Platform module
DROP TABLE IF EXISTS `data_dictionary`;
DROP TABLE IF EXISTS `audit_logs`;
DROP TABLE IF EXISTS `operation_logs`;
DROP TABLE IF EXISTS `tenants`;

-- IDP module
DROP TABLE IF EXISTS `idp_wechat_apps`;

-- Authz module
DROP TABLE IF EXISTS `casbin_rule`;
DROP TABLE IF EXISTS `authz_policy_versions`;
DROP TABLE IF EXISTS `authz_assignments`;
DROP TABLE IF EXISTS `authz_roles`;
DROP TABLE IF EXISTS `authz_resources`;

-- Authn module
DROP TABLE IF EXISTS `jwks_keys`;
DROP TABLE IF EXISTS `auth_token_audit`;
DROP TABLE IF EXISTS `auth_credentials`;
DROP TABLE IF EXISTS `auth_accounts`;

-- UC module
DROP TABLE IF EXISTS `guardianships`;
DROP TABLE IF EXISTS `children`;
DROP TABLE IF EXISTS `users`;
