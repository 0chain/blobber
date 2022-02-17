\connect blobber_meta;

CREATE OR REPLACE FUNCTION update_modified_column() 
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW; 
END;
$$ language 'plpgsql';


CREATE TABLE allocations(
    id VARCHAR (64) PRIMARY KEY,
    tx VARCHAR (64) NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    used_size BIGINT NOT NULL DEFAULT 0,
    owner_id VARCHAR(64) NOT NULL,
    payer_id VARCHAR(64) NOT NULL,
    repairer_id VARCHAR(64) NOT NULL,
    owner_public_key VARCHAR(512) NOT NULL,
    expiration_date BIGINT NOT NULL,
    allocation_root VARCHAR(255) NOT NULL DEFAULT '',
    blobber_size BIGINT NOT NULL DEFAULT 0,
    blobber_size_used BIGINT NOT NULL DEFAULT 0,
    latest_redeemed_write_marker VARCHAR(255),
    is_redeem_required BOOLEAN,
    is_immutable BOOLEAN NOT NULL,
    cleaned_up BOOLEAN NOT NULL DEFAULT FALSE,
    finalized BOOLEAN NOT NULL DEFAULT FALSE,
    time_unit BIGINT NOT NULL DEFAULT 172800000000000,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TRIGGER allocation_modtime BEFORE UPDATE ON allocations FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();

CREATE TABLE allocation_connections(
    connection_id VARCHAR (64) PRIMARY KEY,
    allocation_id VARCHAR(64) NOT NULL,
    client_id VARCHAR(64) NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    status INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TRIGGER allocation_connections_modtime BEFORE UPDATE ON allocation_connections FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();


CREATE TABLE allocation_changes(
    id BIGSERIAL PRIMARY KEY,
    connection_id VARCHAR (64) REFERENCES allocation_connections(connection_id),
    operation VARCHAR(64) NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    input TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TRIGGER allocation_changes_modtime BEFORE UPDATE ON allocation_changes FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();

CREATE TABLE terms (
    id             bigserial,
    blobber_id     varchar(64) NOT NULL,
    allocation_id  varchar(64) REFERENCES allocations (id),
    read_price     bigint NOT NULL,
    write_price    bigint NOT NULL,

    PRIMARY KEY (id)
);

-- clients' pending reads / writes
CREATE TABLE pendings (
    id             VARCHAR(129) PRIMARY KEY, -- combination of client_id:allocation_id
    pending_write  bigint NOT NULL DEFAULT 0, -- number of pending write bytes
    pending_read bigint NOT NULL DEFAULT 0, -- number of pending read bytes
    PRIMARY KEY (id)
);

CREATE TABLE read_pools (
    pool_id        text NOT NULL, -- unique
    client_id      varchar(64) NOT NULL,
    -- blobber_id     varchar(64) NOT NULL,
    allocation_id  varchar(64) NOT NULL,
    balance        bigint NOT NULL,
    expire_at      bigint NOT NULL,

    PRIMARY KEY (pool_id)
);

CREATE TABLE write_pools (
    pool_id        text NOT NULL, -- unique
    client_id      varchar(64) NOT NULL,
    -- blobber_id     varchar(64) NOT NULL,
    allocation_id  varchar(64) NOT NULL,
    balance        bigint NOT NULL,
    expire_at      bigint NOT NULL,

    PRIMARY KEY (pool_id)
);

CREATE TABLE reference_objects (
    id BIGSERIAL PRIMARY KEY,
    lookup_hash VARCHAR (64) NOT NULL,
    path_hash VARCHAR (64) NOT NULL,
    type VARCHAR(10) NOT NULL,
    allocation_id VARCHAR(64) NOT NULL,
    name VARCHAR(100) NOT NULL,
    path TEXT NOT NULL,
    hash VARCHAR(64) NOT NULL,
    num_of_blocks BIGINT NOT NULL DEFAULT 0,
    parent_path TEXT,
    level INT NOT NULL DEFAULT 0,
    custom_meta TEXT NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    chunk_size INT NOT NULL DEFAULT 65536,
    merkle_root VARCHAR(64) NOT NULL,
    actual_file_size BIGINT NOT NULL DEFAULT 0,
    actual_file_hash VARCHAR(64) NOT NULL,
    mimetype VARCHAR(64) NOT NULL,
    write_marker VARCHAR(64) NOT NULL,
    thumbnail_hash VARCHAR(64) NOT NULL,
    thumbnail_size BIGINT NOT NULL DEFAULT 0,
    actual_thumbnail_size BIGINT NOT NULL DEFAULT 0,
    actual_thumbnail_hash VARCHAR(64) NOT NULL,
    encrypted_key TEXT,
    attributes JSON DEFAULT '{}'::jsonb,
    on_cloud BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE TRIGGER reference_objects_modtime BEFORE UPDATE ON reference_objects FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();

CREATE TABLE write_markers (
    allocation_root VARCHAR (64) PRIMARY KEY,
    prev_allocation_root VARCHAR (64) NOT NULL,
    allocation_id VARCHAR(64) NOT NULL,
    client_id VARCHAR(64) NOT NULL,
    blobber_id VARCHAR(64) NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    timestamp BIGINT NOT NULL,
    signature VARCHAR(256) NOT NULL,
    status INT NOT NULL DEFAULT 0,
    status_message TEXT,
    redeem_retries INT NOT NULL DEFAULT 0,
    close_txn_id VARCHAR(64),
    connection_id VARCHAR(64) NOT NULL,
    client_key VARCHAR(512) NOT NULL,
    sequence BIGSERIAL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TRIGGER write_markers_modtime BEFORE UPDATE ON write_markers FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();

CREATE TABLE read_markers (
    id INT PRIMARY KEY,
    client_id VARCHAR(64) NOT NULL,
    client_public_key VARCHAR(128) NOT NULL,
    -- blobber_id VARCHAR(64) NOT NULL,
    allocation_id VARCHAR(64) NOT NULL,
    owner_id VARCHAR(64) NOT NULL,
    payer_id VARCHAR(64) NOT NULL,
    auth_ticket JSON,
    timestamp BIGINT NOT NULL,
    read_blocks INT NOT NULL,
    is_suspend Boolean,
    signature VARCHAR(64) NOT NULL,
    latest_redeemed_rm JSON,
    redeem_required boolean,
    latest_redeem_txn_id VARCHAR(64),
    status_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TRIGGER read_markers_modtime BEFORE UPDATE ON read_markers FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();

CREATE TABLE challenges (
    challenge_id VARCHAR(64) NOT NULL PRIMARY KEY,
    prev_challenge_id VARCHAR(64),
    seed BIGINT NOT NULL DEFAULT 0,
    allocation_id VARCHAR(64) NOT NULL,
    allocation_root VARCHAR(255),
    responded_allocation_root VARCHAR(255),
    status INT NOT NULL DEFAULT 0,
    result INT NOT NULL DEFAULT 0,
    status_message TEXT,
    commit_txn_id VARCHAR(64),
    block_num BIGINT,
    ref_id BIGINT,
    validation_tickets JSON,
    validators JSON,
    last_commit_txn_ids JSON,
    object_path JSON,
    sequence BIGSERIAL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TRIGGER challenges_modtime BEFORE UPDATE ON challenges FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();


CREATE TABLE file_stats (
    id BIGSERIAL PRIMARY KEY,
    ref_id BIGINT UNIQUE REFERENCES reference_objects(id),
    num_of_updates BIGINT,
    num_of_block_downloads BIGINT,
    num_of_challenges BIGINT,
    num_of_failed_challenges BIGINT,
    last_challenge_txn VARCHAR(64),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TRIGGER file_stats_modtime BEFORE UPDATE ON file_stats FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();

CREATE TABLE commit_meta_txns (
    ref_id BIGSERIAL NOT NULL,
    txn_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE collaborators (
    ref_id BIGSERIAL NOT NULL,
    client_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE marketplace_share_info (
    id BIGSERIAL PRIMARY KEY,
    owner_id VARCHAR(64) NOT NULL,
    client_id VARCHAR(64) NOT NULL,
    file_path_hash TEXT NOT NULL,
    re_encryption_key TEXT NOT NULL,
    client_encryption_public_key TEXT NOT NULL,
    expiry_at TIMESTAMP NULL,
    revoked BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TRIGGER share_info_modtime BEFORE UPDATE ON marketplace_share_info FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();
