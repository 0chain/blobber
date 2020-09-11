\connect blobber_meta;

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
ALTER TABLE read_markers
    ADD COLUMN suspend BIGINT NOT NULL DEFAULT -1;
