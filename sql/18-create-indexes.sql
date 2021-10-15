\connect blobber_meta;
BEGIN;
-- Create index on path column; It cannot be Unique index because of soft delete by gorm
CREATE INDEX path_idx ON reference_objects (path);
CREATE INDEX update_idx ON reference_objects (updated_at);
COMMIT;