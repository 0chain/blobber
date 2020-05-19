\connect blobber_meta;

ALTER TABLE allocations ADD COLUMN under_repair boolean NOT NULL DEFAULT false;

ALTER TABLE allocations ADD COLUMN payer_id VARCHAR(64) NOT NULL;
