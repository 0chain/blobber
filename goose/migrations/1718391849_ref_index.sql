-- +goose Up
-- +goose StatementBegin
DROP INDEX idx_created_at,idx_updated_at,idx_path_gin_trgm,idx_name_gin,idx_allocation_changes_lookup_hash;

CREATE INDEX idx_is_precommit on reference_objects(allocation_id) INCLUDE(type,lookup_hash,id) WHERE is_precommit=true AND type='f' AND deleted_at IS NULL;

CREATE INDEX idx_is_deleted on reference_objects(allocation_id) WHERE deleted_at IS NOT NULL;
-- +goose StatementEnd