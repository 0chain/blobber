\connect blobber_meta;


CREATE TABLE marketplace (
    private_key VARCHAR(512) NOT NULL,
    public_key VARCHAR(512) NOT NULL,
    mnemonic VARCHAR(512) NOT NULL
);

GRANT ALL PRIVILEGES ON TABLE marketplace TO blobber_user;
