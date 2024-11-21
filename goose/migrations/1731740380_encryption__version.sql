-- +goose Up
-- +goose StatementBegin

ALTER TABLE reference_objects ADD COLUMN encryption_version smallint;

-- +goose StatementEnd