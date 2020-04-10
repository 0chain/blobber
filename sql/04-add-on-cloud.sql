\connect blobber_meta;

ALTER TABLE reference_objects ADD COLUMN on_cloud BOOLEAN DEFAULT FALSE;