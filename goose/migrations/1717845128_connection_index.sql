-- +goose Up
-- +goose StatementBegin

ALTER TABLE allocation_changes DROP CONSTRAINT allocation_changes_pkey CASCADE,
ADD CONSTRAINT allocation_changes_pkey PRIMARY KEY (lookup_hash);
-- +goose StatementEnd