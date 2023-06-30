-- +goose Up
-- +goose StatementBegin

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;


--
-- Name: allocation_changes; Type: TABLE; Schema: public; Owner: blobber_user
--

CREATE TABLE public.allocation_changes (
    id bigint NOT NULL,
    size bigint DEFAULT 0 NOT NULL,
    operation character varying(20) NOT NULL,
    connection_id character varying(64) NOT NULL,
    input text,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);


ALTER TABLE public.allocation_changes OWNER TO blobber_user;

--
-- Name: allocation_changes_id_seq; Type: SEQUENCE; Schema: public; Owner: blobber_user
--

CREATE SEQUENCE public.allocation_changes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.allocation_changes_id_seq OWNER TO blobber_user;

--
-- Name: allocation_changes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: blobber_user
--

ALTER SEQUENCE public.allocation_changes_id_seq OWNED BY public.allocation_changes.id;


--
-- Name: allocation_connections; Type: TABLE; Schema: public; Owner: blobber_user
--

CREATE TABLE public.allocation_connections (
    id text NOT NULL,
    allocation_id character varying(64) NOT NULL,
    client_id character varying(64) NOT NULL,
    size bigint DEFAULT 0 NOT NULL,
    status bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);


ALTER TABLE public.allocation_connections OWNER TO blobber_user;

--
-- Name: allocations; Type: TABLE; Schema: public; Owner: blobber_user
--

CREATE TABLE public.allocations (
    id character varying(64) NOT NULL,
    tx character varying(64) NOT NULL,
    size bigint DEFAULT 0 NOT NULL,
    used_size bigint DEFAULT 0 NOT NULL,
    owner_id character varying(64) NOT NULL,
    owner_public_key character varying(512) NOT NULL,
    repairer_id character varying(64) NOT NULL,
    expiration_date bigint NOT NULL,
    allocation_root character varying(64) DEFAULT ''::character varying NOT NULL,
    file_meta_root character varying(64) DEFAULT ''::character varying NOT NULL,
    blobber_size bigint DEFAULT 0 NOT NULL,
    blobber_size_used bigint DEFAULT 0 NOT NULL,
    latest_redeemed_write_marker character varying(64),
    is_redeem_required boolean,
    time_unit bigint DEFAULT '172800000000000'::bigint NOT NULL,
    cleaned_up boolean DEFAULT false NOT NULL,
    finalized boolean DEFAULT false NOT NULL,
    file_options integer DEFAULT 63 NOT NULL
);


ALTER TABLE public.allocations OWNER TO blobber_user;

--
-- Name: challenge_timing; Type: TABLE; Schema: public; Owner: blobber_user
--

CREATE TABLE public.challenge_timing (
    challenge_id character varying(64) NOT NULL,
    created_at_chain bigint,
    created_at_blobber bigint,
    file_size bigint,
    proof_gen_time bigint,
    complete_validation bigint,
    txn_submission bigint,
    txn_verification bigint,
    cancelled bigint,
    expiration bigint,
    closed_at bigint,
    updated_at bigint
);


ALTER TABLE public.challenge_timing OWNER TO blobber_user;

--
-- Name: challenges; Type: TABLE; Schema: public; Owner: blobber_user
--

CREATE TABLE public.challenges (
    challenge_id character varying(64) NOT NULL,
    prev_challenge_id character varying(64),
    seed bigint DEFAULT 0 NOT NULL,
    allocation_id text NOT NULL,
    allocation_root character varying(64),
    responded_allocation_root character varying(64),
    status integer DEFAULT 0 NOT NULL,
    result integer DEFAULT 0 NOT NULL,
    status_message text,
    commit_txn_id character varying(64),
    block_num bigint,
    validators jsonb,
    validation_tickets jsonb,
    last_commit_txn_ids jsonb,
    ref_id bigint,
    object_path jsonb,
    sequence bigint,
    "timestamp" bigint DEFAULT 0 NOT NULL,
    created_at bigint,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


ALTER TABLE public.challenges OWNER TO blobber_user;

--
-- Name: challenges_sequence_seq; Type: SEQUENCE; Schema: public; Owner: blobber_user
--

CREATE SEQUENCE public.challenges_sequence_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.challenges_sequence_seq OWNER TO blobber_user;

--
-- Name: challenges_sequence_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: blobber_user
--

ALTER SEQUENCE public.challenges_sequence_seq OWNED BY public.challenges.sequence;


--
-- Name: file_stats; Type: TABLE; Schema: public; Owner: blobber_user
--

CREATE TABLE public.file_stats (
    id bigint NOT NULL,
    ref_id bigint,
    num_of_updates bigint,
    num_of_block_downloads bigint,
    num_of_challenges bigint,
    num_of_failed_challenges bigint,
    last_challenge_txn character varying(64),
    deleted_at timestamp with time zone,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);


ALTER TABLE public.file_stats OWNER TO blobber_user;

--
-- Name: file_stats_id_seq; Type: SEQUENCE; Schema: public; Owner: blobber_user
--

CREATE SEQUENCE public.file_stats_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.file_stats_id_seq OWNER TO blobber_user;

--
-- Name: file_stats_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: blobber_user
--

ALTER SEQUENCE public.file_stats_id_seq OWNED BY public.file_stats.id;


--
-- Name: marketplace_share_info; Type: TABLE; Schema: public; Owner: blobber_user
--

CREATE TABLE public.marketplace_share_info (
    id bigint NOT NULL,
    owner_id character varying(64) NOT NULL,
    client_id character varying(64) NOT NULL,
    file_path_hash character varying(64) NOT NULL,
    re_encryption_key text NOT NULL,
    client_encryption_public_key text NOT NULL,
    revoked boolean NOT NULL,
    expiry_at timestamp with time zone NOT NULL,
    available_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


ALTER TABLE public.marketplace_share_info OWNER TO blobber_user;

--
-- Name: marketplace_share_info_id_seq; Type: SEQUENCE; Schema: public; Owner: blobber_user
--

CREATE SEQUENCE public.marketplace_share_info_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.marketplace_share_info_id_seq OWNER TO blobber_user;

--
-- Name: marketplace_share_info_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: blobber_user
--

ALTER SEQUENCE public.marketplace_share_info_id_seq OWNED BY public.marketplace_share_info.id;


--
-- Name: pendings; Type: TABLE; Schema: public; Owner: blobber_user
--

CREATE TABLE public.pendings (
    id text NOT NULL,
    pending_write bigint DEFAULT 0 NOT NULL
);


ALTER TABLE public.pendings OWNER TO blobber_user;

--
-- Name: read_markers; Type: TABLE; Schema: public; Owner: blobber_user
--

CREATE TABLE public.read_markers (
    client_id character varying(64) NOT NULL,
    allocation_id character varying(64) NOT NULL,
    client_public_key character varying(128),
    owner_id character varying(64),
    "timestamp" bigint,
    counter bigint,
    signature character varying(64),
    session_rc bigint,
    latest_redeemed_rc bigint,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);


ALTER TABLE public.read_markers OWNER TO blobber_user;

--
-- Name: read_pools; Type: TABLE; Schema: public; Owner: blobber_user
--

CREATE TABLE public.read_pools (
    client_id character varying(64) NOT NULL,
    balance bigint NOT NULL
);


ALTER TABLE public.read_pools OWNER TO blobber_user;

--
-- Name: reference_objects; Type: TABLE; Schema: public; Owner: blobber_user
--

CREATE TABLE public.reference_objects (
    id bigint NOT NULL,
    file_id text,
    type character varying(1),
    allocation_id character varying(64) NOT NULL,
    lookup_hash character varying(64) NOT NULL,
    name character varying(100) NOT NULL,
    thumbnail_filename text,
    path character varying(1000) NOT NULL COLLATE pg_catalog."POSIX",
    file_meta_hash character varying(64) NOT NULL,
    hash character varying(64) NOT NULL,
    num_of_blocks bigint DEFAULT 0 NOT NULL,
    path_hash character varying(64) NOT NULL,
    parent_path character varying(999),
    level bigint DEFAULT 0 NOT NULL,
    custom_meta text NOT NULL,
    validation_root character varying(64) NOT NULL,
    prev_validation_root text,
    validation_root_signature character varying(64),
    size bigint DEFAULT 0 NOT NULL,
    fixed_merkle_root character varying(64) NOT NULL,
    actual_file_size bigint DEFAULT 0 NOT NULL,
    actual_file_hash_signature character varying(64),
    actual_file_hash character varying(64) NOT NULL,
    mimetype character varying(64) NOT NULL,
    allocation_root character varying(64) NOT NULL,
    thumbnail_size bigint DEFAULT 0 NOT NULL,
    thumbnail_hash character varying(64) NOT NULL,
    prev_thumbnail_hash text,
    actual_thumbnail_size bigint DEFAULT 0 NOT NULL,
    actual_thumbnail_hash character varying(64) NOT NULL,
    encrypted_key character varying(64),
    created_at bigint,
    updated_at bigint,
    deleted_at timestamp with time zone,
    is_precommit boolean DEFAULT false NOT NULL,
    chunk_size bigint DEFAULT 65536 NOT NULL
);


ALTER TABLE public.reference_objects OWNER TO blobber_user;

--
-- Name: reference_objects_id_seq; Type: SEQUENCE; Schema: public; Owner: blobber_user
--

CREATE SEQUENCE public.reference_objects_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.reference_objects_id_seq OWNER TO blobber_user;

--
-- Name: reference_objects_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: blobber_user
--

ALTER SEQUENCE public.reference_objects_id_seq OWNED BY public.reference_objects.id;


--
-- Name: settings; Type: TABLE; Schema: public; Owner: blobber_user
--

CREATE TABLE public.settings (
    id character varying(10) NOT NULL,
    capacity bigint DEFAULT 0 NOT NULL,
    min_lock_demand numeric DEFAULT 0.000000 NOT NULL,
    num_delegates bigint DEFAULT 100 NOT NULL,
    read_price numeric DEFAULT 0.000000 NOT NULL,
    write_price numeric DEFAULT 0.000000 NOT NULL,
    service_charge numeric DEFAULT 0.000000 NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


ALTER TABLE public.settings OWNER TO blobber_user;

--
-- Name: terms; Type: TABLE; Schema: public; Owner: blobber_user
--

CREATE TABLE public.terms (
    id bigint NOT NULL,
    blobber_id character varying(64) NOT NULL,
    allocation_id character varying(64) NOT NULL,
    read_price bigint NOT NULL,
    write_price bigint NOT NULL
);


ALTER TABLE public.terms OWNER TO blobber_user;

--
-- Name: terms_id_seq; Type: SEQUENCE; Schema: public; Owner: blobber_user
--

CREATE SEQUENCE public.terms_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.terms_id_seq OWNER TO blobber_user;

--
-- Name: terms_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: blobber_user
--

ALTER SEQUENCE public.terms_id_seq OWNED BY public.terms.id;


--
-- Name: write_locks; Type: TABLE; Schema: public; Owner: blobber_user
--

CREATE TABLE public.write_locks (
    allocation_id character varying(64) NOT NULL,
    connection_id character varying(64),
    created_at timestamp with time zone
);


ALTER TABLE public.write_locks OWNER TO blobber_user;

--
-- Name: write_markers; Type: TABLE; Schema: public; Owner: blobber_user
--

CREATE TABLE public.write_markers (
    allocation_root character varying(64) NOT NULL,
    prev_allocation_root character varying(64),
    file_meta_root character varying(64),
    allocation_id character varying(64),
    size bigint,
    blobber_id character varying(64),
    "timestamp" bigint NOT NULL,
    client_id character varying(64),
    signature character varying(64),
    status bigint DEFAULT 0 NOT NULL,
    status_message text,
    redeem_retries bigint DEFAULT 0 NOT NULL,
    close_txn_id character varying(64),
    connection_id character varying(64),
    client_key character varying(256),
    sequence bigint,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);


ALTER TABLE public.write_markers OWNER TO blobber_user;

--
-- Name: write_markers_sequence_seq; Type: SEQUENCE; Schema: public; Owner: blobber_user
--

CREATE SEQUENCE public.write_markers_sequence_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.write_markers_sequence_seq OWNER TO blobber_user;

--
-- Name: write_markers_sequence_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: blobber_user
--

ALTER SEQUENCE public.write_markers_sequence_seq OWNED BY public.write_markers.sequence;


--
-- Name: write_pools; Type: TABLE; Schema: public; Owner: blobber_user
--

CREATE TABLE public.write_pools (
    allocation_id character varying(64) NOT NULL,
    balance bigint NOT NULL
);


ALTER TABLE public.write_pools OWNER TO blobber_user;

--
-- Name: allocation_changes id; Type: DEFAULT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.allocation_changes ALTER COLUMN id SET DEFAULT nextval('public.allocation_changes_id_seq'::regclass);


--
-- Name: challenges sequence; Type: DEFAULT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.challenges ALTER COLUMN sequence SET DEFAULT nextval('public.challenges_sequence_seq'::regclass);


--
-- Name: file_stats id; Type: DEFAULT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.file_stats ALTER COLUMN id SET DEFAULT nextval('public.file_stats_id_seq'::regclass);


--
-- Name: marketplace_share_info id; Type: DEFAULT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.marketplace_share_info ALTER COLUMN id SET DEFAULT nextval('public.marketplace_share_info_id_seq'::regclass);


--
-- Name: reference_objects id; Type: DEFAULT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.reference_objects ALTER COLUMN id SET DEFAULT nextval('public.reference_objects_id_seq'::regclass);


--
-- Name: terms id; Type: DEFAULT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.terms ALTER COLUMN id SET DEFAULT nextval('public.terms_id_seq'::regclass);


--
-- Name: write_markers sequence; Type: DEFAULT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.write_markers ALTER COLUMN sequence SET DEFAULT nextval('public.write_markers_sequence_seq'::regclass);


--
-- Name: allocation_changes allocation_changes_pkey; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.allocation_changes
    ADD CONSTRAINT allocation_changes_pkey PRIMARY KEY (id);


--
-- Name: allocation_connections allocation_connections_pkey; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.allocation_connections
    ADD CONSTRAINT allocation_connections_pkey PRIMARY KEY (id);


--
-- Name: allocations allocations_pkey; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.allocations
    ADD CONSTRAINT allocations_pkey PRIMARY KEY (id);


--
-- Name: allocations allocations_tx_key; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.allocations
    ADD CONSTRAINT allocations_tx_key UNIQUE (tx);


--
-- Name: challenge_timing challenge_timing_pkey; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.challenge_timing
    ADD CONSTRAINT challenge_timing_pkey PRIMARY KEY (challenge_id);


--
-- Name: challenges challenges_pkey; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.challenges
    ADD CONSTRAINT challenges_pkey PRIMARY KEY (challenge_id);


--
-- Name: challenges challenges_sequence_key; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.challenges
    ADD CONSTRAINT challenges_sequence_key UNIQUE (sequence);


--
-- Name: file_stats file_stats_pkey; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.file_stats
    ADD CONSTRAINT file_stats_pkey PRIMARY KEY (id);


--
-- Name: file_stats file_stats_ref_id_key; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.file_stats
    ADD CONSTRAINT file_stats_ref_id_key UNIQUE (ref_id);


--
-- Name: marketplace_share_info marketplace_share_info_pkey; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.marketplace_share_info
    ADD CONSTRAINT marketplace_share_info_pkey PRIMARY KEY (id);


--
-- Name: pendings pendings_pkey; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.pendings
    ADD CONSTRAINT pendings_pkey PRIMARY KEY (id);


--
-- Name: read_markers read_markers_pkey; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.read_markers
    ADD CONSTRAINT read_markers_pkey PRIMARY KEY (client_id, allocation_id);


--
-- Name: read_pools read_pools_pkey; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.read_pools
    ADD CONSTRAINT read_pools_pkey PRIMARY KEY (client_id);


--
-- Name: reference_objects reference_objects_pkey; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.reference_objects
    ADD CONSTRAINT reference_objects_pkey PRIMARY KEY (id);


--
-- Name: settings settings_pkey; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.settings
    ADD CONSTRAINT settings_pkey PRIMARY KEY (id);


--
-- Name: terms terms_pkey; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.terms
    ADD CONSTRAINT terms_pkey PRIMARY KEY (id);


--
-- Name: write_locks write_locks_pkey; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.write_locks
    ADD CONSTRAINT write_locks_pkey PRIMARY KEY (allocation_id);


--
-- Name: write_markers write_markers_pkey; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.write_markers
    ADD CONSTRAINT write_markers_pkey PRIMARY KEY (allocation_root, "timestamp");


--
-- Name: write_markers write_markers_sequence_key; Type: CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.write_markers
    ADD CONSTRAINT write_markers_sequence_key UNIQUE (sequence);


--
-- Name: idx_closed_at; Type: INDEX; Schema: public; Owner: blobber_user
--

CREATE INDEX idx_closed_at ON public.challenge_timing USING btree (closed_at DESC);


--
-- Name: idx_created_at; Type: INDEX; Schema: public; Owner: blobber_user
--

CREATE INDEX idx_created_at ON public.reference_objects USING btree (created_at DESC);


--
-- Name: idx_lookup_hash_alloc; Type: INDEX; Schema: public; Owner: blobber_user
--

CREATE INDEX idx_lookup_hash_alloc ON public.reference_objects USING btree (allocation_id, lookup_hash);


--
-- Name: idx_marketplace_share_info_for_client; Type: INDEX; Schema: public; Owner: blobber_user
--

CREATE INDEX idx_marketplace_share_info_for_client ON public.marketplace_share_info USING btree (client_id, file_path_hash);


--
-- Name: idx_marketplace_share_info_for_owner; Type: INDEX; Schema: public; Owner: blobber_user
--

CREATE INDEX idx_marketplace_share_info_for_owner ON public.marketplace_share_info USING btree (owner_id, file_path_hash);


--
-- Name: idx_name_gin:gin; Type: INDEX; Schema: public; Owner: blobber_user
--

CREATE INDEX "idx_name_gin:gin" ON public.reference_objects USING btree (name);


--
-- Name: idx_path_alloc; Type: INDEX; Schema: public; Owner: blobber_user
--

CREATE INDEX idx_path_alloc ON public.reference_objects USING btree (allocation_id, path);


--
-- Name: idx_seq; Type: INDEX; Schema: public; Owner: blobber_user
--

CREATE UNIQUE INDEX idx_seq ON public.write_markers USING btree (allocation_id, sequence);


--
-- Name: idx_status; Type: INDEX; Schema: public; Owner: blobber_user
--

CREATE INDEX idx_status ON public.challenges USING btree (status);


--
-- Name: idx_unique_allocations_tx; Type: INDEX; Schema: public; Owner: blobber_user
--

CREATE UNIQUE INDEX idx_unique_allocations_tx ON public.allocations USING btree (tx);


--
-- Name: idx_updated_at; Type: INDEX; Schema: public; Owner: blobber_user
--

CREATE INDEX idx_updated_at ON public.reference_objects USING btree (updated_at DESC);


--
-- Name: idx_write_pools_cab; Type: INDEX; Schema: public; Owner: blobber_user
--

CREATE INDEX idx_write_pools_cab ON public.write_pools USING btree (allocation_id);


--
-- Name: path_idx; Type: INDEX; Schema: public; Owner: blobber_user
--

CREATE INDEX path_idx ON public.reference_objects USING btree (path);


--
-- Name: allocation_changes fk_allocation_connections_changes; Type: FK CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.allocation_changes
    ADD CONSTRAINT fk_allocation_connections_changes FOREIGN KEY (connection_id) REFERENCES public.allocation_connections(id);


--
-- Name: file_stats fk_file_stats_ref; Type: FK CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.file_stats
    ADD CONSTRAINT fk_file_stats_ref FOREIGN KEY (ref_id) REFERENCES public.reference_objects(id) ON DELETE CASCADE;


--
-- Name: terms fk_terms_allocation; Type: FK CONSTRAINT; Schema: public; Owner: blobber_user
--

ALTER TABLE ONLY public.terms
    ADD CONSTRAINT fk_terms_allocation FOREIGN KEY (allocation_id) REFERENCES public.allocations(id);


--
-- PostgreSQL database dump complete
--

-- +goose StatementEnd
