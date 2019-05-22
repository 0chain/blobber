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
    size BIGINT NOT NULL DEFAULT 0,
    used_size BIGINT NOT NULL DEFAULT 0,
    owner_id VARCHAR(64) NOT NULL,
    owner_public_key VARCHAR(256) NOT NULL,
    expiration_date BIGINT NOT NULL,
    allocation_root VARCHAR(255),
    blobber_size BIGINT NOT NULL DEFAULT 0,
    blobber_size_used BIGINT NOT NULL DEFAULT 0,
    latest_redeemed_write_marker VARCHAR(255),
    is_redeem_required BOOLEAN,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TRIGGER allocation_modtime BEFORE UPDATE ON allocations FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();
CREATE OR REPLACE FUNCTION update_write_redeem_required_column() 
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.allocation_root != NEW.latest_redeemed_write_marker THEN
        NEW.is_redeem_required = true;
    ELSE
        NEW.is_redeem_required = false;
    END IF;
    RETURN NEW; 
END;
$$ language 'plpgsql';
CREATE TRIGGER write_markers_redeem_required BEFORE UPDATE ON allocations FOR EACH ROW EXECUTE PROCEDURE  update_write_redeem_required_column();


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

CREATE TABLE reference_objects (
    id BIGSERIAL PRIMARY KEY,
    path_hash VARCHAR (64) UNIQUE,
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
    merkle_root VARCHAR(64) NOT NULL,
    actual_file_size BIGINT NOT NULL DEFAULT 0,
    actual_file_hash VARCHAR(64) NOT NULL,
    write_marker VARCHAR(64) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
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
    client_key VARCHAR(256) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TRIGGER write_markers_modtime BEFORE UPDATE ON write_markers FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();

CREATE TABLE read_markers (
    client_id VARCHAR(64) NOT NULL PRIMARY KEY,
    client_public_key VARCHAR(256) NOT NULL,
    blobber_id VARCHAR(64) NOT NULL,
    allocation_id VARCHAR(64) NOT NULL,
    owner_id VARCHAR(64) NOT NULL,
    timestamp BIGINT NOT NULL,
    counter BIGINT NOT NULL DEFAULT 0,
    signature VARCHAR(256) NOT NULL,
    latest_redeemed_rm JSON,
    redeem_required boolean,
    latest_redeem_txn_id VARCHAR(64),
    status_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TRIGGER read_markers_modtime BEFORE UPDATE ON read_markers FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();
CREATE OR REPLACE FUNCTION update_read_redeem_required_column() 
RETURNS TRIGGER AS $$
DECLARE
    ctr    BIGINT := 0;
    redeem_counter VARCHAR(50) := '0';
BEGIN
    IF NEW.latest_redeemed_rm IS NOT NULL THEN
		select INTO redeem_counter NEW.latest_redeemed_rm ->> 'counter';
		select INTO ctr CAST(redeem_counter AS BIGINT);
	END IF;	
    IF ctr < NEW.counter THEN
        NEW.redeem_required = true;
    ELSE
        NEW.redeem_required = false;
    END IF;
    RETURN NEW; 
END;
$$ language 'plpgsql';
CREATE TRIGGER read_markers_redeem_required BEFORE UPDATE ON read_markers FOR EACH ROW EXECUTE PROCEDURE  update_read_redeem_required_column();