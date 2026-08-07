-- schema_version is redundant metadata with no lossless inverse after removal.
SIGNAL SQLSTATE '45000'
    SET MESSAGE_TEXT = '000020 schema_version retirement is irreversible; restore a verified backup instead';
