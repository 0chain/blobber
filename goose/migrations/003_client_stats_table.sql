-- +goose Up

--
-- Name: client_stats; Type: TABLE; Schema: public; Owner: blobber_user
--

CREATE TABLE public.client_stats(
    client_id character varying(64),
    created_at bigint DEFAULT 0 NOT NULL,
    total_upload bigint DEFAULT 0 NOT NULL,
    total_download bigint DEFAULT 0 NOT NULL,
    total_write_marker bigint DEFAULT 0 NOT NULL
)

-- +goose StatementBegin

ALTER TABLE public.client_stats OWNER TO blobber_user;

ALTER TABLE ONLY public.client_stats
    ADD CONSTRAINT client_stats_pkey PRIMARY KEY (client_id, created_at);
--
-- PostgreSQL database dump complete
--
-- +goose StatementEnd