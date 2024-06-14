-- +goose Up
-- +goose StatementBegin
ALTER TABLE ONLY terms DROP CONSTRAINT fk_terms_allocation;
ALTER TABLE ONLY terms ADD CONSTRAINT fk_terms_allocation foreign key (allocation_id) references allocations(id) ON DELETE CASCADE;
CREATE INDEX idx_terms_allocation_id ON terms USING btree(allocation_id);
-- +goose StatementEnd