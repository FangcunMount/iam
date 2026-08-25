-- Intentionally irreversible. Restoring legacy authorization facts requires
-- the verified pre-cutover database backup and the matching old binaries.
SIGNAL SQLSTATE '45000'
    SET MESSAGE_TEXT = 'migration 000027 is irreversible; restore the pre-cutover backup';
