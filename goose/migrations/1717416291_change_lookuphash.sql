-- +goose Up
-- +goose StatementBegin
ALTER TABLE allocation_changes ADD COLUMN lookup_hash character varying(64);

-- CREATE UNIQUE INDEX idx_allocation_changes_lookup_hash ON allocation_changes USING HASH(lookup_hash,connection_id);
-- +goose StatementEnd