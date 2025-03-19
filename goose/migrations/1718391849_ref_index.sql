-- +goose Up
-- +goose StatementBegin
DROP INDEX idx_created_at,idx_updated_at,idx_path_gin_trgm,idx_name_gin;

CREATE INDEX idx_is_allocation_version_deleted_at on reference_objects(allocation_id,allocation_version) WHERE type='f' AND deleted_at IS NULL;

CREATE INDEX idx_is_deleted on reference_objects(allocation_id) WHERE deleted_at IS NOT NULL;

CREATE INDEX idx_path_alloc_level ON reference_objects USING btree (allocation_id,level,type,path) WHERE deleted_at IS NULL;
-- +goose StatementEnd