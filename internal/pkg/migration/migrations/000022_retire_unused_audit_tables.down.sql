-- The retired log/audit tables contained no production rows and have no inverse.
SIGNAL SQLSTATE '45000'
    SET MESSAGE_TEXT = '000022 audit table retirement is irreversible; restore a verified backup instead';
