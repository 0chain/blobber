-- +goose Up
-- +goose StatementBegin

CREATE INDEX idx_allocation_changes_lookup_hash ON allocation_changes USING HASH(lookup_hash);
-- +goose StatementEnd