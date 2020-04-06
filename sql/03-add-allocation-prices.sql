\connect blobber_meta;

ALTER TABLE allocations ADD COLUMN read_price   double precision NOT NULL DEFAULT 0;
ALTER TABLE allocations ADD COLUMN write_price  double precision NOT NULL DEFAULT 0;
ALTER TABLE allocations ADD COLUMN num_blobbers bigint NOT NULL DEFAULT 1;
