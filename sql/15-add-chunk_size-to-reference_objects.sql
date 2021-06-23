--
-- Add chunk_size column to reference_objects table.
--

-- pew-pew
\connect blobber_meta;

-- in a transaction
BEGIN;
    ALTER TABLE reference_objects
        ADD COLUMN chunk_size INT NOT NULL DEFAULT 65536;
COMMIT;