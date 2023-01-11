#!/bin/bash

CONFIG_DIR=$HOME/.zcn
MINIO_USERNAME=manali_user
MINIO_PASSWORD=manali_password
ALLOCATION=837419df249882e67a7d1d3746671fe1d93c35f988e3eb0fd2a0a4aec24f50b5
BLOCK_WORKER_URL=https://dev.0chain.net/dns
LOCK_TOKENS=1
MINIO_TOKEN=1234

sudo apt update
sudo apt install -y unzip curl containerd docker.io

echo "download docker-compose"
sudo curl -L "https://github.com/docker/compose/releases/download/1.29.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
docker-compose --version

echo "download zwallet CLI"
sudo curl -L "https://github.com/0chain/zwalletcli/releases/download/v1.1.6/zwallet-linux.tar.gz" -o /tmp/zwallet-linux.tar.gz
tar -C /usr/local/bin -xvf /tmp/zwallet-linux.tar.gz

echo "download zbox CLI"
sudo curl -L "https://github.com/0chain/zboxcli/releases/download/v1.4.0/zbox-linux.tar.gz" -o /tmp/zbox-linux.tar.gz
tar -C /usr/local/bin -xvf /tmp/zbox-linux.tar.gz

# create config dir
mkdir -p $CONFIG_DIR

# create config.yaml
cat <<EOF >${CONFIG_DIR}/config.yaml
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

# when: create_wallet == "yes"
zwallet --configDir ${CONFIG_DIR} faucet --methodName pour --input "new wallet"
zwallet --configDir ${CONFIG_DIR} faucet --methodName pour --input "{Pay day}"

# when: create_allocation == "yes"
zbox --configDir ${CONFIG_DIR} newallocation --lock ${LOCK_TOKENS}

cat <<EOF >${CONFIG_DIR}/allocation.txt
$ALLOCATION
EOF

# create docker-compose
cat <<EOF >${CONFIG_DIR}/docker-compose.yml
version: '3.8'
services:
  db:
    image: postgres:13-alpine
    container_name: postgres-db
    restart: always
    command: -c "log_statement=all"
    environment:
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
    ports:
      - '5432:5432'
    volumes:
      - db:/var/lib/postgresql/data

  api:
    image: 0chaindev/blimp-logsearchapi:v0.0.2
    depends_on:
      - db
    ports:
      - 8080:8080
    environment:
      LOGSEARCH_PG_CONN_STR: "postgres://postgres:postgres@postgres-db/postgres?sslmode=disable"
      LOGSEARCH_AUDIT_AUTH_TOKEN: ${MINIO_TOKEN}
      MINIO_LOG_QUERY_AUTH_TOKEN: ${MINIO_TOKEN}
      LOGSEARCH_DISK_CAPACITY_GB: 5
    links:
      - db

  minioserver:
    image: 0chaindev/blimp-minioserver:v0.0.1
    container_name: minioserver
    ports:
      - 9000:9000
    command: ["minio", "gateway", "zcn"]
    environment:
      MINIO_AUDIT_WEBHOOK_ENDPOINT: http://api:8080/api/ingest?token=${MINIO_TOKEN}
      MINIO_AUDIT_WEBHOOK_AUTH_TOKEN: ${MINIO_TOKEN}
      MINIO_AUDIT_WEBHOOK_ENABLE: "on"
      MINIO_ROOT_USER: ${MINIO_USERNAME}
      MINIO_ROOT_PASSWORD: ${MINIO_PASSWORD}
      MINIO_BROWSER: "OFF"
    links:
      - api:api
    volumes:
      - ${CONFIG_DIR}:/root/.zcn

  minioclient:
    image: 0chaindev/blimp-clientapi:v0.0.2
    container_name: minioclient
    depends_on:
      - minioserver
    ports:
      - 3001:3001
    environment:
      MINIO_SERVER: "minioserver:9000"

volumes:
  db:
    driver: local
EOF

sudo docker-compose -f ${CONFIG_DIR}/docker-compose.yml up -d
