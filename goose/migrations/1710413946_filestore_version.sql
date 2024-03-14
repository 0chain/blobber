-- +goose Up
-- +goose StatementBegin
ALTER TABLE reference_objects ADD COLUMN filestore_version INTEGER;
-- +goose StatementEnd