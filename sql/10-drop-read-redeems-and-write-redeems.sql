\connect blobber_meta;

--
-- don't track every redeem, since update allocation or slow redeeming can
-- break process; in such cases it requires workarounds; use pendings table
-- to track pending redeems
--

DROP TABLE read_redeems CASCADE;
DROP TABLE write_redeems CASCADE; -- with indices
