#!/bin/bash

MIGRATION_ROOT=$HOME/s3migration
ACCESS_KEY=0chainaccesskey
SECRET_KEY=0chainsecretkey
ALLOCATION=0chainallocation
BUCKET=0chainbucket
# ALLOCATION=1467bdcedb20e62928eb18960c501c1c50ff35c927b86b34a8f0b00eb29a6d06
# BUCKET=cloud-mig
BLIMP_DOMAIN=blimpdomain

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
CONFIG_DIR=$HOME/.zcn

BLOCK_WORKER_URL=0chainblockworker
# BLOCK_WORKER_URL=https://helm.0chain.net/dns
sudo apt update
sudo apt install -y unzip curl containerd docker.io jq

echo "download docker-compose"
sudo curl -L "https://github.com/docker/compose/releases/download/1.29.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
docker-compose --version

sudo curl -L "https://s3-mig-binaries.s3.us-east-2.amazonaws.com/s3mgrt" -o /usr/local/bin/s3mgrt
chmod +x /usr/local/bin/s3mgrt

mkdir -p ${MIGRATION_ROOT}

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

# conform if the wallet belongs to an allocationID

_contains () {  # Check if space-separated list $1 contains line $2
  echo "$1" | tr ' ' '\n' | grep -F -x -q "$2"
}

allocations=$(zbox listallocations --silent --json | jq -r ' .[] | .id')

if ! _contains "${allocations}" "${ALLOCATION}"; then
  echo "given allocation does not belong to the wallet"
  exit 1
fi

cat <<EOF >${CONFIG_DIR}/allocation.txt
$ALLOCATION
EOF

cat <<EOF >${CONFIG_DIR}/Caddyfile
${BLIMP_DOMAIN}:9012 {
	route {
		reverse_proxy s3mgrt:8080
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
      - 9012:9012
    volumes:
      - ${CONFIG_DIR}/Caddyfile:/etc/caddy/Caddyfile
      - ${CONFIG_DIR}/caddy/site:/srv
      - ${CONFIG_DIR}/caddy/caddy_data:/data
      - ${CONFIG_DIR}/caddy/caddy_config:/config
    restart: "always"

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
