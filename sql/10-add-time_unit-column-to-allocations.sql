--
-- Add column time_unit to allocations. Default is 48h.
--

BEGIN;
    ALTER TABLE allocations
        ADD COLUMN time_unit BIGINT NOT NULL DEFAULT 172800000000000;
COMMIT;
