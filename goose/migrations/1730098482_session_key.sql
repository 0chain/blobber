-- +goose Up
-- +goose StatementBegin
ALTER TABLE allocations ADD COLUMN owner_signing_public_key character varying(512);

ALTER TABLE reference_objects ADD COLUMN signature_version smallint;

-- +goose StatementEnd