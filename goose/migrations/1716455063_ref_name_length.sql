-- +goose Up
-- +goose StatementBegin
ALTER TABLE reference_objects ALTER COLUMN name TYPE character varying(150),
ALTER COLUMN name SET NOT NULL;
-- +goose StatementEnd