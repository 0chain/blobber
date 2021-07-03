\connect blobber_meta;

CREATE INDEX idx_reference_objects_for_lookup_hash ON reference_objects(allocation_id, lookup_hash);
CREATE INDEX idx_reference_objects_for_path ON reference_objects(allocation_id, path);
