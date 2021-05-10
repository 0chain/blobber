\connect blobber_meta;

ALTER TABLE allocations
    ADD COLUMN cleaned_up boolean NOT NULL DEFAULT false;

ALTER TABLE allocations
    ADD COLUMN finalized boolean NOT NULL DEFAULT false;
