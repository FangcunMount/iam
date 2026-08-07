-- The retired Identity tables contained historical facts with no lossless
-- inverse mapping. Recreating empty shells would make the migration version lie.
SIGNAL SQLSTATE '45000'
    SET MESSAGE_TEXT = '000019 legacy Identity table retirement is irreversible; restore a verified backup instead';
