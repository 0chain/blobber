\connect blobber_meta;
BEGIN;
DROP INDEX IF EXISTS idx_unique_allocations_tx;
CREATE UNIQUE INDEX idx_unique_allocations_tx ON allocations (tx);

DROP INDEX IF EXISTS idx_pendings_cab;
CREATE UNIQUE INDEX idx_pendings_cab ON pendings (client_id, allocation_id, blobber_id);

DROP INDEX IF EXISTS idx_read_pools_cab;
CREATE INDEX idx_read_pools_cab ON read_pools (client_id, allocation_id, blobber_id);

DROP INDEX IF EXISTS idx_write_pools_cab;
CREATE INDEX idx_write_pools_cab ON write_pools (client_id, allocation_id, blobber_id);

-- TODO one of path_idx and idx_reference_objects_for_path is redundant
-- Create index on path column; It cannot be Unique index because of soft delete by gorm
DROP INDEX IF EXISTS path_idx;
CREATE INDEX path_idx ON reference_objects (path);

DROP INDEX IF EXISTS update_idx;
CREATE INDEX update_idx ON reference_objects (updated_at);

DROP INDEX IF EXISTS idx_reference_objects_for_lookup_hash;
CREATE INDEX idx_reference_objects_for_lookup_hash ON reference_objects(allocation_id, lookup_hash);

DROP INDEX IF EXISTS idx_reference_objects_for_path;
CREATE INDEX idx_reference_objects_for_path ON reference_objects(allocation_id, path);

DROP INDEX IF EXISTS idx_marketplace_share_info_for_owner;
CREATE INDEX idx_marketplace_share_info_for_owner ON marketplace_share_info(owner_id, file_path_hash);

DROP INDEX IF EXISTS idx_marketplace_share_info_for_client;
CREATE INDEX idx_marketplace_share_info_for_client ON marketplace_share_info(client_id, file_path_hash);

COMMIT;