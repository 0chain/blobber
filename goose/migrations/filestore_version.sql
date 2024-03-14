-- +goose Up
-- +goose StatementBegin
ALTER TABLE reference_objects ADD COLUMN filestore_version INT;
-- +goose StatementEnd