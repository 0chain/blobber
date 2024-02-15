-- +goose Up
-- +goose StatementBegin
ALTER TABLE write_markers
ADD COLUMN chain_hash character varying(64),
ADD COLUMN chain_size BIGINT,
ADD COLUMN chain_length integer;
-- +goose StatementEnd