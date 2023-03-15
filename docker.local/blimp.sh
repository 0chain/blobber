#!/bin/bash

CONFIG_DIR=$HOME/.zcn
MIGRATION_ROOT=$HOME/.s3migration
MINIO_USERNAME=0chainminiousername
MINIO_PASSWORD=0chainminiopassword
ALLOCATION=0chainallocationid
BLOCK_WORKER_URL=0chainblockworker
# BLOCK_WORKER_URL=https://helm.0chain.net/dns
# todo: check with team
MINIO_TOKEN=0chainminiotoken
BLIMP_DOMAIN=blimpdomain

sudo apt update
sudo apt install -y unzip curl containerd docker.io

echo "download docker-compose"
sudo curl -L "https://github.com/docker/compose/releases/download/1.29.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
docker-compose --version

# create config dir
mkdir -p $CONFIG_DIR

# check if wallet.json file exists
test -f n; echo $?
if [ ! -f ${CONFIG_DIR}/wallet.json ]
then
	echo "wallet.json does not exist in ${CONFIG_DIR}. Exiting..."
	exit 1
fi

# create config.yaml
cat <<EOF >${CONFIG_DIR}/config.yaml
block_worker: ${BLOCK_WORKER_URL}
signature_scheme: bls0chain
min_submit: 50
min_confirmation: 50
confirmation_chain_length: 3
max_txn_query: 5
query_sleep_time: 5
EOF

# todo: how do you conform if the wallet belongs to an allocationID
cat <<EOF >${CONFIG_DIR}/allocation.txt
$ALLOCATION
EOF

cat <<EOF >${CONFIG_DIR}/Caddyfile
${BLIMP_DOMAIN}:3001 {
	route {
		reverse_proxy minioclient:3001
	}
}

${BLIMP_DOMAIN}:8080 {
	route {
		reverse_proxy api:8080
	}
}

${BLIMP_DOMAIN}:9000 {
	route {
		reverse_proxy minioclient:9000
	}
}

EOF


sudo docker-compose -f ${CONFIG_DIR}/docker-compose.yml down

# create docker-compose
cat <<EOF >${CONFIG_DIR}/docker-compose.yml
version: '3.8'
services:
  caddy:
    image: caddy:latest
    ports:
      - 80:80
      - 8080:8080
      - 9000:9000
      - 3001:3001
      - 9012:9012
    volumes:
      - ${CONFIG_DIR}/Caddyfile:/etc/caddy/Caddyfile
      - ${CONFIG_DIR}/caddy/site:/srv
      - ${CONFIG_DIR}/caddy/caddy_data:/data
      - ${CONFIG_DIR}/caddy/caddy_config:/config
    restart: "always"

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
    image: 0chaindev/blimp-logsearchapi:pr-13-6882a858
    depends_on:
      - db
    environment:
      LOGSEARCH_PG_CONN_STR: "postgres://postgres:postgres@postgres-db/postgres?sslmode=disable"
      LOGSEARCH_AUDIT_AUTH_TOKEN: 12345
      MINIO_LOG_QUERY_AUTH_TOKEN: 12345
      LOGSEARCH_DISK_CAPACITY_GB: 5
    links:
      - db

  minioserver:
    image: 0chaindev/blimp-minioserver:pr-13-6882a858
    container_name: minioserver
    command: ["minio", "gateway", "zcn"]
    environment:
      MINIO_AUDIT_WEBHOOK_ENDPOINT: http://api:8080/api/ingest?token=${MINIO_TOKEN}
      MINIO_AUDIT_WEBHOOK_AUTH_TOKEN: 12345
      MINIO_AUDIT_WEBHOOK_ENABLE: "on"
      MINIO_ROOT_USER: manali
      MINIO_ROOT_PASSWORD: manalipassword
      MINIO_BROWSER: "OFF"
    links:
      - api:api
    volumes:
      - ${CONFIG_DIR}:/root/.zcn

  minioclient:
    image: 0chaindev/blimp-clientapi:pr-13-6882a858
    container_name: minioclient
    depends_on:
      - minioserver
    environment:
      MINIO_SERVER: "minioserver:9000"
      
volumes:
  db:
    driver: local

EOF

sudo docker-compose -f ${CONFIG_DIR}/docker-compose.yml up -d
