-- Run once, after reviewing the distribution of failed_attempts and locked_until,
-- immediately before enabling auth.password_lockout in production.
--
-- This preserves active future locks and audit timestamps. It only clears stale
-- failure counts that were accumulated before the lockout policy became active.
UPDATE auth_credentials
SET failed_attempts = 0
WHERE failed_attempts > 0
  AND (locked_until IS NULL OR locked_until <= UTC_TIMESTAMP());
