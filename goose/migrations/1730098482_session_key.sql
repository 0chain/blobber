-- +goose Up
-- +goose StatementBegin
ALTER TABLE allocations ADD COLUMN owner_signing_public_key character varying(512);

ALTER TABLE reference_objects ADD COLUMN signature_version smallint;

ALTER TABLE reference_objects ALTER COLUMN actual_file_hash_signature TYPE character varying(128);

ALTER TABLE reference_objects ALTER COLUMN validation_root_signature TYPE character varying(128);

-- +goose StatementEnd