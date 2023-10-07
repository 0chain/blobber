-- +goose Up

-- Dropping incorrect index on the 'name' column
DROP INDEX IF EXISTS "idx_name_gin:gin";

-- Creating a new GIN index for full-text search on the 'name' column
CREATE INDEX idx_name_gin ON public.reference_objects 
USING gin(to_tsvector('english', name));

-- Creating a new GIN index for trigram matching on the 'path' column
CREATE INDEX idx_path_gin_trgm ON public.reference_objects USING gin(path gin_trgm_ops);

-- +goose Down
DROP INDEX IF EXISTS idx_path_gin_trgm;
DROP INDEX IF EXISTS idx_name_gin;
CREATE INDEX IF NOT EXISTS "idx_name_gin:gin" ON public.reference_objects USING btree (name);
