-- +goose Up
-- +goose StatementBegin
ALTER TABLE allocations ADD COLUMN storage_version smallint;
ALTER TABLE allocations ADD COLUMN num_objects integer;

-- +goose StatementEnd