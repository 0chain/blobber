#!/bin/bash

MIGRATION_ROOT=$HOME/.s3migration
ACCESS_KEY=0chainaccesskey
SECRET_KEY=0chainsecretkey
ALLOCATION=0chainallocation
BUCKET=0chainbucket
# ACCESS_KEY=AKIA4MPQDEZ4OACO45FD
# SECRET_KEY=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
# ALLOCATION=1467bdcedb20e62928eb18960c501c1c50ff35c927b86b34a8f0b00eb29a6d06
# BUCKET=cloud-mig
CONCURRENCY=1
DELETE_SOURCE=false
ENCRYPT=false
REGION=us-east-2
# BLOCK_WORKER_URL=0chainblockworkerurl
BLOCK_WORKER_URL=https://helm.0chain.net/dns
sudo apt update
sudo apt install -y unzip curl containerd docker.io

echo "download docker-compose"
sudo curl -L "https://github.com/docker/compose/releases/download/1.29.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
docker-compose --version

sudo curl -L "https://s3-mig-binaries.s3.us-east-2.amazonaws.com/s3mgrt" -o /usr/local/bin/s3mgrt
chmod +x /usr/local/bin/s3mgrt

mkdir -p ${MIGRATION_ROOT}

cat <<EOF >${MIGRATION_ROOT}/docker-compose.yml
version: '3.8'
services:
  db:
    image: bmanu199/s3mgrt:latest
    restart: always
    ports:
      - '9012:8080'
    volumes:
      - ${MIGRATION_ROOT}:/migrate
EOF

cat <<EOF >${MIGRATION_ROOT}/config.yaml
block_worker: ${BLOCK_WORKER_URL}
signature_scheme: bls0chain
min_submit: 50
min_confirmation: 50
confirmation_chain_length: 3
max_txn_query: 5
query_sleep_time: 5
# # OPTIONAL - Uncomment to use/ Add more if you want
# preferred_blobbers:
#   - http://one.devnet-0chain.net:31051
#   - http://one.devnet-0chain.net:31052
EOF

/usr/local/bin/docker-compose -f ${MIGRATION_ROOT}/docker-compose.yml up -d

#  --concurrency ${CONCURRENCY} --delete-source ${DELETE_SOURCE} --encrypt ${ENCRYPT} --resume true   --skip 1

cd ${MIGRATION_ROOT}
/usr/local/bin/s3mgrt migrate --access-key ${ACCESS_KEY} --secret-key ${SECRET_KEY} --allocation ${ALLOCATION} --bucket ${BUCKET} --region ${REGION}
