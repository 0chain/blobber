--
-- add name columns to allocations table
--

-- pew-pew
\connect blobber_meta;

BEGIN;
    ALTER TABLE allocations ADD COLUMN name VARCHAR(64);
COMMIT;