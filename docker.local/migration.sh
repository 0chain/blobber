#!/bin/bash

MIGRATION_ROOT=$HOME/.s3migration
SCRIPT=0chainmigrationscript

sudo apt update
sudo apt install -y unzip curl containerd docker.io

echo "download docker-compose"
sudo curl -L "https://github.com/docker/compose/releases/download/1.29.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
docker-compose --version

sudo curl -L "https://github.com/0chain/s3-migration/raw/staging/.bin/s3migration" -o /usr/local/bin/s3mgrtmigrate

mkdir -p ${MIGRATION_ROOT}

cat <<EOF >${PROJECT_ROOT}/config/docker-compose.yml
version: '3.8'
services:
  db:
    image: bmanu199/s3mgrt:latest
    restart: always
    ports:
      - '9012:9012'
    volumes:
      - ${MIGRATION_ROOT}:/root/.zcn
EOF

${SCRIPT}
