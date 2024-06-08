-- +goose Up
-- +goose StatementBegin

CREATE UNIQUE INDEX idx_allocation_changes_lookup_hash ON allocation_changes USING HASH(connection_id,lookup_hash);
-- +goose StatementEnd