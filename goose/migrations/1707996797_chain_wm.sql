-- +goose Up
-- +goose StatementBegin
ALTER TABLE write_markers
ADD COLUMN chain_hash character varying(64),
ADD COLUMN chain_size BIGINT,
ADD COLUMN chain_length integer;

ALTER TABLE allocations ADD COLUMN last_redeemed_sequence BIGINT DEFAULT 0;
-- +goose StatementEnd