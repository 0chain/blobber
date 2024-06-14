-- +goose Up
-- +goose StatementBegin
DROP INDEX idx_created_at,idx_updated_at,idx_lookup_hash,path_idx;
CREATE UNIQUE INDEX idx_lookup_deleted_at ON reference_objects USING btree(lookup_hash,deleted_at);
-- +goose StatementEnd