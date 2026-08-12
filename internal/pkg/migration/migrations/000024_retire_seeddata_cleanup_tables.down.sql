-- The four one-off cleanup/backup tables cannot be reconstructed from the
-- canonical Identity model. Restore the verified pre-release backup instead.
SIGNAL SQLSTATE '45000'
    SET MESSAGE_TEXT = '000024 seeddata cleanup table retirement is irreversible; restore a verified backup instead';
