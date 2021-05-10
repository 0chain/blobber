\connect blobber_meta;

ALTER TABLE allocations ADD COLUMN payer_id VARCHAR(64) NOT NULL;

