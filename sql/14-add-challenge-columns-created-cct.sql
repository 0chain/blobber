--
-- add payer_id and auth_tiket columns to read_markers table
--

-- pew-pew
\connect blobber_meta;

BEGIN;
    ALTER TABLE challenges ADD COLUMN created_at INT(30) NOT NULL;
    ALTER TABLE challenges ADD COLUMN cct INT(30) NOT NULL ;
COMMIT;