\connect blobber_meta;


CREATE TABLE marketplace_share_info (
    id BIGSERIAL PRIMARY KEY,
    owner_id VARCHAR(64) NOT NULL,
    client_id VARCHAR(64) NOT NULL,
    file_path_hash TEXT NOT NULL,
    re_encryption_key TEXT NOT NULL,
    client_encryption_public_key TEXT NOT NULL,
    expiry_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_marketplace_share_info_for_owner ON marketplace_share_info(owner_id, file_path_hash);
CREATE INDEX idx_marketplace_share_info_for_client ON marketplace_share_info(client_id, file_path_hash);

CREATE TRIGGER share_info_modtime BEFORE UPDATE ON marketplace_share_info FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();

GRANT ALL PRIVILEGES ON TABLE marketplace_share_info TO blobber_user;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO blobber_user;
