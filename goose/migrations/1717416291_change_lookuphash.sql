-- +goose Up
-- +goose StatementBegin
ALTER TABLE allocation_changes ADD COLUMN lookup_hash character varying(64);

-- +goose StatementEnd