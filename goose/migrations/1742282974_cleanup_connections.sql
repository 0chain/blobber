-- +goose Up
-- +goose StatementBegin

DELETE FROM allocation_connections WHERE status = 2 OR status = 3;

-- +goose StatementEnd