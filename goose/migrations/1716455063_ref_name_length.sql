-- +goose Up
-- +goose StatementBegin
ALTER TABLE reference_objects ALTER COLUMN name TYPE character varying(150) NOT NULL;
-- +goose StatementEnd