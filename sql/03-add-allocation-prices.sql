\connect blobber_meta;

ALTER TABLE allocations ADD COLUMN tx varchar (64) NOT NULL;

CREATE UNIQUE INDEX idx_unique_allocations_tx ON allocations (tx);

CREATE TABLE terms (
    id             bigserial,

    blobber_id     varchar(64) NOT NULL,
    allocation_tx  varchar(64) REFERENCES allocations (tx),

    read_price     bigint NOT NULL,
    write_price    bigint NOT NULL,

    PRIMARY KEY (id)
);

-- clients' pending reads / writes
CREATE TABLE pendings (
    id             bigserial,

    client_id      varchar(64) NOT NULL,
    allocation_id  varchar(64) NOT NULL,
    blobber_id     varchar(64) NOT NULL,

    pending_read   bigint NOT NULL DEFAULT 0,
    pending_write  bigint NOT NULL DEFAULT 0,

    PRIMARY KEY (id)
);

CREATE UNIQUE INDEX idx_pendings_cab
    ON pendings (client_id, allocation_id, blobber_id);

CREATE TABLE read_pools (
    pool_id        text NOT NULL, -- unique

    client_id      varchar(64) NOT NULL,
    blobber_id     varchar(64) NOT NULL,
    allocation_id  varchar(64) NOT NULL,

    balance        bigint NOT NULL,
    expire_at      bigint NOT NULL,

    PRIMARY KEY (pool_id)
);

CREATE UNIQUE INDEX idx_read_pools_cab
    ON read_pools (client_id, allocation_id, blobber_id);

CREATE TABLE write_pools (
    pool_id        text NOT NULL, -- unique

    client_id      varchar(64) NOT NULL,
    blobber_id     varchar(64) NOT NULL,
    allocation_id  varchar(64) NOT NULL,

    balance        bigint NOT NULL,
    expire_at      bigint NOT NULL,

    PRIMARY KEY (pool_id)
);

CREATE UNIQUE INDEX idx_write_pools_cab
    ON write_pools (client_id, allocation_id, blobber_id);


ALTER TABLE read_markers ADD COLUMN
    num_blocks bigint NOT NULL;

CREATE TABLE read_redeems (
    id             bigserial,

    read_counter   bigint NOT NULL,
    value          bigint NOT NULL,

    client_id      varchar(64) NOT NULL,
    blobber_id     varchar(64) NOT NULL,
    allocation_id  varchar(64) NOT NULL,

    PRIMARY KEY (id)
);

CREATE TABLE write_redeems (
    id             bigserial,

    signature      varchar(256) NOT NULL, -- write marker signature

    size           bigint NOT NULL,
    value          bigint NOT NULL,

    client_id      varchar(64) NOT NULL,
    blobber_id     varchar(64) NOT NULL,
    allocation_id  varchar(64) NOT NULL,

    PRIMARY KEY (id)
);

CREATE INDEX idx_write_redeems_signature ON write_redeems (signature);
