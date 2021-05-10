--
-- add payer_id and auth_tiket columns to read_markers table
--

-- pew-pew
\connect blobber_meta;

BEGIN;
    ALTER TABLE read_markers ADD COLUMN payer_id VARCHAR(64) NOT NULL;
    ALTER TABLE read_markers ADD COLUMN auth_ticket JSON;
COMMIT;
