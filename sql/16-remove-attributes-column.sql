--
-- Add who_pays column to reference_objects table.
--

-- pew-pew
\connect blobber_meta;

-- in a transaction
BEGIN;
ALTER TABLE reference_objects
    DROP COLUMN attributes;
COMMIT;
