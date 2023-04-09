#!/bin/bash

CONFIG_DIR=$HOME/.zcn
CONFIG_DIR_BLIMP=${CONFIG_DIR}/blimp # to store wallet.json, config.json, allocation.json
MIGRATION_ROOT=$HOME/.s3migration
MINIO_USERNAME=0chainminiousername
MINIO_PASSWORD=0chainminiopassword
ALLOCATION=0chainallocationid
BLOCK_WORKER_URL=0chainblockworker
MINIO_TOKEN=0chainminiotoken
BLIMP_DOMAIN=blimpdomain
WALLET_ID=0chainwalletid
WALLET_PUBLIC_KEY=0chainwalletpublickey
WALLET_PRIVATE_KEY=0chainwalletprivatekey

sudo apt update
sudo apt install -y unzip curl containerd docker.io jq

echo "download docker-compose"
sudo curl -L "https://github.com/docker/compose/releases/download/1.29.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
docker-compose --version

# create config dir
mkdir -p ${CONFIG_DIR}
mkdir -p ${CONFIG_DIR_BLIMP}

cat <<EOF >${CONFIG_DIR_BLIMP}/wallet.json
{
  "client_id": "${WALLET_ID}",
  "client_key": "${WALLET_PUBLIC_KEY}",
  "keys": [
    {
      "public_key": "${WALLET_PUBLIC_KEY}",
      "private_key": "${WALLET_PRIVATE_KEY}"
    }
  ],
  "version": "1.0"
}
EOF

# create config.yaml
cat <<EOF >${CONFIG_DIR_BLIMP}/config.yaml
block_worker: ${BLOCK_WORKER_URL}
signature_scheme: bls0chain
min_submit: 50
min_confirmation: 50
confirmation_chain_length: 3
max_txn_query: 5
query_sleep_time: 5
EOF

# conform if the wallet belongs to an allocationID

_contains () {  # Check if space-separated list $1 contains line $2
  echo "$1" | tr ' ' '\n' | grep -F -x -q "$2"
}

allocations=$(zbox listallocations --configDir ${CONFIG_DIR_BLIMP} --silent --json | jq -r ' .[] | .id')

if ! _contains "${allocations}" "${ALLOCATION}"; then
  echo "given allocation does not belong to the wallet"
  exit 1
fi

# todo: verify if updating the allocation ID causes issues to the existing deployment
cat <<EOF >${CONFIG_DIR_BLIMP}/allocation.txt
$ALLOCATION
EOF

# create a seperate folder to store caddy files
mkdir -p ${CONFIG_DIR}/caddyfiles

cat <<EOF > ${CONFIG_DIR}/caddyfiles/Caddyfile
import /etc/caddy/*.caddy
EOF

cat <<EOF >${CONFIG_DIR}/caddyfiles/blimp.caddy
${BLIMP_DOMAIN} {
	route /minioclient/* {
		uri strip_prefix /minioclient
		reverse_proxy minioclient:3001
	}

	route /logsearch/* {
		uri strip_prefix /logsearch
		reverse_proxy api:8080
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
      - 443:443
    volumes:
      - ${CONFIG_DIR}/caddyfiles:/etc/caddy
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
    image: 0chaindev/blimp-minioserver:pr-18-0dd2027e
    container_name: minioserver
    command: ["minio", "gateway", "zcn"]
    environment:
      MINIO_AUDIT_WEBHOOK_ENDPOINT: http://api:8080/api/ingest?token=${MINIO_TOKEN}
      MINIO_AUDIT_WEBHOOK_AUTH_TOKEN: 12345
      MINIO_AUDIT_WEBHOOK_ENABLE: "on"
      MINIO_ROOT_USER: ${MINIO_USERNAME}
      MINIO_ROOT_PASSWORD: ${MINIO_PASSWORD}
      MINIO_BROWSER: "OFF"
    links:
      - api:api
    volumes:
      - ${CONFIG_DIR_BLIMP}:/root/.zcn

  minioclient:
    image: 0chaindev/blimp-clientapi:pr-13-6882a858
    container_name: minioclient
    depends_on:
      - minioserver
    environment:
      MINIO_SERVER: "minioserver:9000"

  s3mgrt:
    image: bmanu199/s3mgrt:latest
    restart: always
    volumes:
      - ${MIGRATION_ROOT}:/migrate

volumes:
  db:
    driver: local

EOF

sudo docker-compose -f ${CONFIG_DIR}/docker-compose.yml up -d
