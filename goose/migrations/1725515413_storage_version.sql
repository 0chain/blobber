-- +goose Up
-- +goose StatementBegin
ALTER TABLE allocations ADD COLUMN storage_version smallint;
ALTER TABLE allocations ADD COLUMN num_objects integer;
ALTER TABLE allocations ADD COLUMN prev_num_objects integer;
ALTER TABLE allocations ADD COLUMN num_blocks bigint;
ALTER TABLE allocations ADD COLUMN prev_num_blocks bigint;

DROP INDEX IF EXISTS idx_parent_path_alloc,idx_path_alloc;
ALTER TABLE reference_objects DROP CONSTRAINT path_commit;

--
-- Name: idx_parent_path_alloc; Type: INDEX; Schema: public; Owner: blobber_user
--

CREATE INDEX idx_parent_path_alloc ON reference_objects USING btree (allocation_id, parent_path) WHERE deleted_at IS NULL;


--
-- Name: idx_path_alloc; Type: INDEX; Schema: public; Owner: blobber_user
--

CREATE INDEX idx_path_alloc ON reference_objects USING btree (allocation_id, path) WHERE deleted_at IS NULL;

CREATE INDEX idx_is_deleted ON reference_objects(allocation_id) WHERE deleted_at is NOT NULL;

CREATE INDEX idx_is_allocation_root_deleted_at ON reference_objects(allocation_id, allocation_root) WHERE type='f' AND deleted_at IS NULL;

CREATE INDEX idx_path_alloc_level ON reference_objects USING btree (allocation_id,level,type,path) WHERE deleted_at IS NULL;

ALTER TABLE ONLY allocation_changes ADD CONSTRAINT connection_id_lookup_hash UNIQUE (connection_id,lookup_hash);

CREATE UNIQUE INDEX idx_lookup_hash_deleted ON reference_objects(lookup_hash,(deleted_at IS NULL)) INCLUDE(id,type,num_of_updates,size);

-- +goose StatementEnd