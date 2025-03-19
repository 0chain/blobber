-- +goose Up
-- +goose StatementBegin

 --
 -- Name: connection_id_lookup_hash; Type: UNIQUE CONSTRAINT; Schema: public; Owner: blobber_user
 --   

ALTER TABLE ONLY allocation_changes ADD CONSTRAINT connection_id_lookup_hash UNIQUE(connection_id,lookup_hash);

-- +goose StatementEnd