-- +goose Up
-- +goose StatementBegin
DELETE FROM terms WHERE allocation_id NOT IN (SELECT id FROM allocations);
ALTER TABLE ONLY terms DROP CONSTRAINT fk_terms_allocation;
ALTER TABLE ONLY terms ADD CONSTRAINT fk_terms_allocation foreign key (allocation_id) references allocations(id) ON DELETE CASCADE;
CREATE INDEX idx_terms_allocation_id ON terms USING HASH(allocation_id);
-- +goose StatementEnd