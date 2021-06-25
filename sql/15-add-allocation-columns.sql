\connect blobber_meta;

BEGIN;
    ALTER TABLE allocations ADD COLUMN repairer_id VARCHAR(64) NOT NULL;
    ALTER TABLE allocations ADD COLUMN is_immutable BOOLEAN;
COMMIT;