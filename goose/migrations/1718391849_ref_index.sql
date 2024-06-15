-- +goose Up
-- +goose StatementBegin
DROP INDEX idx_created_at,idx_updated_at,idx_lookup_hash,path_idx,idx_path_gin_trgm,idx_name_gin,idx_path_alloc;
CREATE INDEX idx_allocation_path ON reference_objects USING btree(allocation_id,path,level);
-- +goose StatementEnd