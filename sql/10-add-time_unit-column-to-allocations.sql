--
-- Add column time_unit to allocations.
--

BEGIN;
    ALTER TABLE allocations
        ADD COLUMN time_unit BIGINT NOT NULL DEFAULT 0;
COMMIT;
