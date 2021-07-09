--
-- Add chunk_size column to reference_objects table.
--

-- pew-pew
\connect blobber_meta;


ALTER TABLE reference_objects ADD COLUMN chunk_size INT NOT NULL DEFAULT 65536;
