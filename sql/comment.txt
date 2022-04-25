-- \connect blobber_meta;

-- BEGIN;

    -- if the suspend is equal to 'counter' column, then we don't send the marker
    -- even if its 'redeem_required' column is set to true; for a case, where we
    -- can't redeem a read_marker due to 'not enough tokens in related read pools'
    -- then we have to suspend redeeming to avoid 0chain DDoSing; in this case user
    -- can't read more files until his lock more tokens; and when hi locks more
    -- tokens, and reads a file, then the counter will be increased and a suspended
    -- read  marker will be awoken and redeemed
    --
    -- the case we can't redeem a read marker is a slow redeeming; there is
    -- 'read_lock_timeout' configuration, that means an expected timeout between
    -- a reading pre-redeeming and its redeeming; if redeeming works slow for a
    -- reason; then we can try to redeem a read_maker from read_pools already
    -- expired
    --

    -- Changes have been moved to table creation
    -- ALTER TABLE read_markers
    --     ADD COLUMN suspend BIGINT NOT NULL DEFAULT -1;

    --
    -- we don't need to track pending reads anymore
    --
    -- Changes have been moved to table creation
    -- ALTER TABLE pendings
    --     DROP COLUMN pending_read;

    --
    -- pending values has changed from tokens to number of block for read markers
    -- and size in bytes for write markers; we have to reset all of them to zero
    -- to avoid 'tokens * tokens' multiplication in pending tokens calculations
    -- (instead of 'size * tokens' or 'numBlocks * tokens')
    --

    -- (we are doing it in transaction to make sure it called once; since the
    -- alter table above, and drop table below fails next time and rolls back
    -- the transaction; thus the pending will not be reset next time, that's
    -- expected)

    -- UPDATE pendings SET pending_write = 0

    --
    -- don't track every redeem, since update allocation or slow redeeming can
    -- break process; in such cases it requires workarounds; use pendings table
    -- to track pending redeems
    --

    -- Changes have been moved to table creation; Table is simply never created
    -- DROP TABLE read_redeems CASCADE;
    -- DROP TABLE write_redeems CASCADE; -- with indices

-- COMMIT;
