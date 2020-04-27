\connect blobber_meta;

CREATE TABLE commit_meta_txns (
    ref_id BIGSERIAL NOT NULL,
    txn_id VARCHAR(64) NOT NULL
);