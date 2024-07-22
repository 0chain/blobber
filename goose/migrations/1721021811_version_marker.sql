-- +goose Up
-- +goose StatementBegin

CREATE TABLE version_markers(
    id bigint NOT NULL,
    allocation_id character varying(64) NOT NULL,
    blobber_id character varying(64) NOT NULL,
    client_id character varying(64) NOT NULL,
    "version" bigint NOT NULL,
    "timestamp" bigint NOT NULL,
    signature character varying(64), 
    is_repair boolean NOT NULL DEFAULT false,
    repair_version bigint,
    repair_offset character varying(1000)
);

ALTER TABLE version_markers OWNER TO blobber_user;

--
-- Name: version_markers_id_seq; Type: SEQUENCE; Schema: public; Owner: blobber_user
--

CREATE SEQUENCE version_markers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 100;

ALTER TABLE version_markers_id_seq OWNER TO blobber_user;


--
-- Name: version_markers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: blobber_user
--

ALTER SEQUENCE version_markers_id_seq OWNED BY version_markers.id;


--
-- Name: version_markers version_markers_pkey; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY version_markers
    ADD CONSTRAINT version_markers_pkey PRIMARY KEY (id);


--
-- Name: version_markers id; Type: DEFAULT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY version_markers ALTER COLUMN id SET DEFAULT nextval('version_markers_id_seq'::regclass);


--
-- Name: version_markers_allocation_id_idx; Type: INDEX; Schema: public; Owner: blobber_user
--

CREATE INDEX version_markers_allocation_id_idx ON version_markers USING btree (allocation_id,version);

-- +goose StatementEnd
