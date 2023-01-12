#!/bin/bash

MIGRATION_ROOT=$HOME/.s3migration
SCRIPT=0chainmigrationscript

mkdir -p ${MIGRATION_ROOT}

cat <<EOF >${PROJECT_ROOT}/config/docker-compose.yml
version: '3.8'
services:
  db:
    image: postgres:13-alpine
    restart: always
    command: -c "log_statement=all"
    environment:
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
    ports:
      - '5432:5432'
    volumes:
      - ${MIGRATION_ROOT}:/root/.zcn
EOF

${SCRIPT}
