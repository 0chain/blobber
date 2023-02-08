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
BLIMP_DOMAIN=0chainblimpdomain

CONCURRENCY=1
DELETE_SOURCE=0chaindeletesource
ENCRYPT=0chainencrypt
REGION=0chainregion
SKIP=0chainskip
NEWER_THAN=0chainnewerthan
OLDER_THAN=0chainolderthan
PREFIX=0chainprefix
RESUME=0chainresume
MIGRATE_TO=0chainmigrateto
WORKING_DIR=0chainwd

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
CONFIG_DIR=$HOME/.zcn
mkdir -p ${MIGRATION_ROOT}

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
${BLIMP_DOMAIN}:9012 {
	route {
		reverse_proxy s3mgrt:8080
	}
}
EOF

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
    image: 0chaindev/blimp-logsearchapi:v0.0.3
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
    image: 0chaindev/blimp-minioserver:v0.0.1
    container_name: minioserver
    command: ["minio", "gateway", "zcn"]
    environment:
      MINIO_AUDIT_WEBHOOK_ENDPOINT: http://api:8080/api/ingest?token=12345
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
    image: 0chaindev/blimp-clientapi:v0.0.3
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

/usr/local/bin/docker-compose -f ${CONFIG_DIR}/docker-compose.yml up -d

#  --concurrency ${CONCURRENCY} --delete-source ${DELETE_SOURCE} --encrypt ${ENCRYPT} --resume true   --skip 1

flags="--access-key ${ACCESS_KEY} --secret-key ${SECRET_KEY} --allocation ${ALLOCATION} --bucket ${BUCKET} "

# setup optional parameters
if [ $ENCRYPT == "true" ]; then flags=$flags" --encrypt true"; fi
if [ $DELETE_SOURCE == "true" ]; then flags=$flags" --delete-source true"; fi
if [ $REGION != "0chainregion" ]; then flags=$flags"--region ${REGION}"; fi
if [ $SKIP != "0chainskip" ]; then flags=$flags" --skip ${SKIP}"; fi
if [ $NEWER_THAN != "0chainnewerthan" ]; then flags=$flags" --newer-than ${SKIP}"; fi
if [ $OLDER_THAN != "0chainolderthan" ]; then flags=$flags" --older-than ${SKIP}"; fi
if [ $PREFIX != "0chainprefix" ]; then flags=$flags" --prefix ${PREFIX}"; fi
if [ $RESUME == "true" ]; then flags=$flags" --resume ${RESUME}"; fi
if [ $MIGRATE_TO != "0chainmigrateto" ]; then flags=$flags" --migrate-to ${MIGRATE_TO}"; fi
if [ $WORKING_DIR != "0chainwd" ]; then flags=$flags" --wd ${WORKING_DIR}"; fi

cd ${MIGRATION_ROOT}
/usr/local/bin/s3mgrt migrate $flags