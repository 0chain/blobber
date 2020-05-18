\connect blobber_meta;

ALTER TABLE allocations
    ADD COLUMN under_repair boolean NOT NULL DEFAULT false;
