\connect blobber_meta;

CREATE TABLE collaborators (
    ref_id BIGSERIAL NOT NULL,
    client_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);