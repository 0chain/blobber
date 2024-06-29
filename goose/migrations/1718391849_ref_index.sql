-- +goose Up
-- +goose StatementBegin
DROP INDEX idx_created_at,idx_updated_at,idx_lookup_hash,idx_path_gin_trgm,idx_name_gin,idx_allocation_changes_lookup_hash;
-- +goose StatementEnd