-- +goose Up
-- +goose StatementBegin
SET idle_in_transaction_session_timeout = 180000;
-- +goose StatementEnd
