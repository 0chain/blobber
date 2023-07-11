CREATE EXTENSION IF NOT EXISTS "pg_trgm";
SET log_statement = 'all';
CREATE DATABASE blobber_meta;
\connect blobber_meta;
CREATE USER blobber_user WITH ENCRYPTED PASSWORD 'blobber';
GRANT ALL PRIVILEGES ON DATABASE blobber_meta TO blobber_user;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO blobber_user;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO blobber_user;