-- +goose Up
-- +goose StatementBegin
DROP INDEX idx_created_at,idx_updated_at,idx_lookup_hash,idx_path_gin_trgm,idx_name_gin,idx_allocation_changes_lookup_hash;

CREATE INDEX idx_allocation_precommit_deleted ON reference_objects (allocation_id, is_precommit) where deleted_at is NULL INCLUDE(type,lookup_hash);
-- +goose StatementEnd