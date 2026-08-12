-- Account-bound AuthN rows have no lossless inverse mapping from the canonical
-- LoginIdentity/Credential model. Restore the verified pre-release backup.
SIGNAL SQLSTATE '45000'
    SET MESSAGE_TEXT = '000023 legacy AuthN table retirement is irreversible; restore a verified backup instead';
