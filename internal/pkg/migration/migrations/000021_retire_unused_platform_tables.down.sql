-- The retired platform tables contained data with no canonical inverse.
SIGNAL SQLSTATE '45000'
    SET MESSAGE_TEXT = '000021 platform table retirement is irreversible; restore a verified backup instead';
