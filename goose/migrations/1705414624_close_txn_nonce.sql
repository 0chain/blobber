-- +goose Up
-- +goose StatementBegin
ALTER TABLE write_markers ADD COLUMN close_txn_nonce BIGINT;
-- +goose StatementEnd