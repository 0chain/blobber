-- +goose Up
-- +goose StatementBegin

ALTER TABLE marketplace_share_info
ADD COLUMN IF NOT EXISTS share_type character varying(16) NOT NULL DEFAULT 'private';

CREATE INDEX IF NOT EXISTS idx_marketplace_share_info_owner_file_type
ON marketplace_share_info (owner_id, file_path_hash, share_type);

CREATE INDEX IF NOT EXISTS idx_marketplace_share_info_client_file_type
ON marketplace_share_info (client_id, file_path_hash, share_type);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_marketplace_share_info_client_file_type;
DROP INDEX IF EXISTS idx_marketplace_share_info_owner_file_type;
ALTER TABLE marketplace_share_info DROP COLUMN IF EXISTS share_type;

-- +goose StatementEnd
