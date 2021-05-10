CREATE extension ltree;
CREATE DATABASE blobber_meta;
\connect blobber_meta;
CREATE USER blobber_user WITH ENCRYPTED PASSWORD 'blobber';
GRANT ALL PRIVILEGES ON DATABASE blobber_meta TO blobber_user;