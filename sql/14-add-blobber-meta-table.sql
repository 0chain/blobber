\connect blobber_meta;


CREATE TABLE blobber_meta (
    id BIGSERIAL NOT NULL,
    private_key VARCHAR(512) NOT NULL,
    public_key VARCHAR(512) NOT NULL
);
