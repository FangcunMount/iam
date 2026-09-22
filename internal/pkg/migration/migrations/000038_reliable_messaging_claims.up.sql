-- Additive schema only. Original writers remain compatible but do not obey the
-- new fencing. Stop/drain legacy Relay owners before enabling SDK claiming.
ALTER TABLE domain_event_outbox
 ADD COLUMN rm_claim_token VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
 ADD COLUMN rm_claim_version BIGINT UNSIGNED NOT NULL DEFAULT 0,
 ADD COLUMN rm_claim_count BIGINT UNSIGNED NOT NULL DEFAULT 0,
 ADD COLUMN rm_lease_until DATETIME(6) NULL,
 ADD COLUMN rm_fingerprint BINARY(32) NULL,
 ADD INDEX idx_outbox_status_rm_lease (status, rm_lease_until),
 ADD INDEX idx_outbox_status_updated (status, updated_at);
