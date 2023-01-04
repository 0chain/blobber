#!/bin/bash

set -e

# setup variables
export CLUSTER=0chaincluster
export DELEGATE_WALLET=0chainclientId
export READ_PRICE=0chainreadPrice
export WRITE_PRICE=0chainwritePrice
# export MIN_STAKE=0chainminStake
# export MAX_STAKE=0chainmaxStake
# export SERVICE_CHARGE=0chainserviceCharge
export MIN_STAKE="1.0"
export MAX_STAKE="100.0"
export SERVICE_CHARGE="0.30"
export GF_ADMIN_USER=0chaingfadminuser
export GF_ADMIN_PASSWORD=0chaingfadminpassword
export PROJECT_ROOT=/var/0chain/blobber
export BLOCK_WORKER_URL=0chainblockworker
export BLOBBER_HOST=0chainblobberhost
# http://${NETWORK}.${DOMAIN}/dns

## cleanup server before starting the deployment
docker-compose -f /var/0chain/blobber/docker-compose.yml down --volumes || true
docker-compose -f /var/0chain/blobber/zchain-compose.yml down --volumes || true
rm -rf /var/0chain/blobber || true

#TODO: Fix docker installation
sudo apt update
sudo apt install -y unzip curl containerd docker.io

# download docker-compose
sudo curl -L "https://github.com/docker/compose/releases/download/1.29.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
docker-compose --version

#### ---- Start Blobber Setup ----- ####

FOLDERS_TO_CREATE="config sql bin monitoringconfig keys_config"

for i in ${FOLDERS_TO_CREATE}; do
    folder=${PROJECT_ROOT}/${i}
    echo "creating folder: $folder"
    mkdir -p $folder
done

ls -al $PROJECT_ROOT

# download and unzip files
curl -L "https://github.com/0chain/blobber/raw/setup-blobber-quickly/docker.local/bin/blobber-files.zip" -o /tmp/blobber-files.zip
unzip -o /tmp/blobber-files.zip -d ${PROJECT_ROOT}
rm /tmp/blobber-files.zip

# create 0chain_blobber.yaml file
echo "creating 0chain_validator.yaml"
cat <<EOF >${PROJECT_ROOT}/config/0chain_blobber.yaml
version: "1.0"

logging:
  level: "info"
  console: true # printing log to console is only supported in development mode

info:
  name: my_blobber
  logo_url: https://google.com
  description: this is my test blobber
  website_url: https://google.com


# for testing
#  500 MB - 536870912
#    1 GB - 1073741824
#    2 GB - 2147483648
#    3 GB - 3221225472
#  100 GB - 107374182400
capacity: 1073741824 # 1 GB bytes total blobber capacity
read_price: ${READ_PRICE}  # token / GB for reading
write_price: ${WRITE_PRICE}    # token / GB / time_unit for writing
price_in_usd: false
price_worker_in_hours: 12
# the time_unit configured in Storage SC and can be given using
#
#     ./zbox sc-config
#

# min_lock_demand is value in [0; 1] range; it represents number of tokens the
# blobber earned even if a user will not read or write something
# to an allocation; the number of tokens will be calculated by the following
# formula (regarding the time_unit and allocation duration)
#
#     allocation_size * write_price * min_lock_demand
#
min_lock_demand: 0.1
# max_offer_duration restrict long contracts where,
# in the future, prices can be changed
max_offer_duration: 744h # 31 day

# these timeouts required by blobber to check client pools, perform
# a task and redeem tokens, it should be big enough
read_lock_timeout: 1m
write_lock_timeout: 1m

# update_allocations_interval used to refresh known allocation objects from SC
update_allocations_interval: 1m

# maximum limit on the number of combined directories and files on each allocation
max_dirs_files: 50000

# delegate wallet (must be set)
delegate_wallet: ${DELEGATE_WALLET}
# min stake allowed, tokens
min_stake: ${MIN_STAKE}
# max stake allowed, tokens
max_stake: ${MAX_STAKE}
# maximum allowed number of stake holders
num_delegates: 50
# service charge of the blobber
service_charge: ${SERVICE_CHARGE}
# min submit from miners
min_submit: 50
# min confirmation from sharder
min_confirmation: 50

block_worker: ${BLOCK_WORKER_URL}

challenge_completion_time: 3m

handlers:
  rate_limit: 0 # 10 per second . it can't too small one if a large file is download with blocks
  file_rate_limit: 100 # 100 files per second

server_chain:
  id: "0afc093ffb509f059c55478bc1a60351cef7b4e9c008a53a6cc8241ca8617dfe"
  owner: "edb90b850f2e7e7cbd0a1fa370fdcc5cd378ffbec95363a7bc0e5a98b8ba5759"
  genesis_block:
    id: "ed79cae70d439c11258236da1dfa6fc550f7cc569768304623e8fbd7d70efae4"
  signature_scheme: "bls0chain"

contentref_cleaner:
  frequency: 30
  tolerance: 3600
openconnection_cleaner:
  frequency: 30
  tolerance: 3600 # 60 * 60
writemarker_redeem:
  frequency: 10
  num_workers: 5
readmarker_redeem:
  frequency: 10
  num_workers: 5
challenge_response:
  frequency: 10
  num_workers: 5
  max_retries: 20

healthcheck:
  frequency: 60s # send healthcheck to miners every 60 seconds

pg:
  user: postgres
  password: postgres
db:
  name: blobber_meta
  user: blobber_user
  password: blobber
  host: postgres
  port: 5432


geolocation:
  latitude: 0
  longitude: 0

storage:
  files_dir: "/path/to/hdd"
#  sha256 hash will have 64 characters of hex encoded length. So if dir_level is [2,2] this means for an allocation id
#  "4c9bad252272bc6e3969be637610d58f3ab2ff8ca336ea2fadd6171fc68fdd56" directory below will be created.
#  alloc_dir = {files_dir}/4c/9b/ad252272bc6e3969be637610d58f3ab2ff8ca336ea2fadd6171fc68fdd56
#
#  So this means, there will maximum of 16^4 = 65536 numbers directories for all allocations stored by blobber.
#  Similarly for some file_hash "ef935503b66b1ce026610edf18bffd756a79676a8fe317d951965b77a77c0227" with dir_level [2, 2, 1]
#  following path is created for the file:
# {alloc_dir}/ef/93/5/503b66b1ce026610edf18bffd756a79676a8fe317d951965b77a77c0227
  alloc_dir_level: [2, 1]
  file_dir_level: [2, 2, 1]

minio:
  # Enable or disable minio backup service
  start: false
  # The frequency at which the worker should look for files, Ex: 3600 means it will run every 3600 seconds
  worker_frequency: 3600 # In Seconds
  # Use SSL for connection or not
  use_ssl: false

  storage_service_url: "play.min.io"
  access_id: "Q3AM3UQ867SPQQA43P2F"
  secret_access_key: "zuf+tfteSlswRu7BJ86wekitnifILbZam1KYY3TG"
  bucket_name: "mytestbucket"
  region: "us-east-1"

cold_storage:
  # Minimum file size to be considered for moving to cloud
  min_file_size: 1048576 #in bytes
  # Minimum time for which file is not updated or not used
  file_time_limit_in_hours: 720 #in hours
  # Number of files to be queried and processed at once
  job_query_limit: 100
  # Capacity filled in bytes after which the cloud backup should start work
  start_capacity_size: 536870912 # 500MB
  # Delete local copy once the file is moved to cloud
  delete_local_copy: true
  # Delete cloud copy if the file is deleted from the blobber by user/other process
  delete_cloud_copy: true

disk_update:
  # defaults to true. If false, blobber has to manually update blobber's capacity upon increase/decrease
  # If blobber has to limit its capacity to 5% of its capacity then it should turn automaci_update to false.
  automatic_update: true
  blobber_update_interval: 5m # In minutes
# integration tests related configurations
integration_tests:
  # address of the server
  address: host.docker.internal:15210
  # lock_interval used by nodes to request server to connect to blockchain
  # after start
  lock_interval: 1s
EOF

### Create 0chain_validator.yaml file
echo "creating 0chain_validator.yaml"
cat <<EOF >${PROJECT_ROOT}/config/0chain_validator.yaml
version: 1.0

# delegate wallet (must be set)
delegate_wallet: ${DELEGATE_WALLET}
# min stake allowed, tokens
min_stake: ${MIN_STAKE}
# max stake allowed, tokens
max_stake: ${MAX_STAKE}
# maximum allowed number of stake holders
num_delegates: 50
# service charge of related blobber
service_charge: ${SERVICE_CHARGE}

block_worker: ${BLOCK_WORKER_URL}

handlers:
  rate_limit: 10 # 10 per second

logging:
  level: "error"
  console: true # printing log to console is only supported in development mode

server_chain:
  id: "0afc093ffb509f059c55478bc1a60351cef7b4e9c008a53a6cc8241ca8617dfe"
  owner: "edb90b850f2e7e7cbd0a1fa370fdcc5cd378ffbec95363a7bc0e5a98b8ba5759"
  genesis_block:
    id: "ed79cae70d439c11258236da1dfa6fc550f7cc569768304623e8fbd7d70efae4"
  network:
    relay_time: 100 # milliseconds
  signature_scheme: "bls0chain"

EOF

### Create minio_config.txt file
echo "creating minio_config.txt"
cat <<EOF >${PROJECT_ROOT}/keys_config/minio_config.txt
block_worker: ${BLOCK_WORKER_URL}
EOF

### Caddyfile
echo "creating Caddyfile"
cat <<EOF >${PROJECT_ROOT}/Caddyfile
${BLOBBER_HOST} {
	log {
		output file /var/log/access.log {
			roll_size 1gb
			roll_keep 5
			roll_keep_for 720h
		}
	}

	route {
		reverse_proxy blobber:5051
	}

	route /monitoring* {
		uri strip_prefix /monitoring
	        header Access-Control-Allow-Methods "POST,PATCH,PUT,DELETE, GET, OPTIONS"
                header Access-Control-Allow-Headers "*"
	        header Access-Control-Allow-Origin "*"
	        header Cache-Control max-age=3600
		reverse_proxy monitoringapi:3001
	}

	route /grafana/* {
		uri strip_prefix /grafana
		reverse_proxy grafana:3000
	}

	redir /grafana /grafana/
	redir /monitoring /monitoring/
}

EOF

### docker-compose.yaml
echo "creating docker-compose file"
cat <<EOF >${PROJECT_ROOT}/docker-compose.yml
---
version: "3"
services:
  postgres:
    image: postgres:14
    environment:
      POSTGRES_HOST_AUTH_METHOD: trust
    volumes:
      - ${PROJECT_ROOT}/data/postgresql:/var/lib/postgresql/data
      - ${PROJECT_ROOT}/postgresql.conf:/var/lib/postgresql/postgresql.conf
    command: postgres -c config_file=/var/lib/postgresql/postgresql.conf
    networks:
      default:
    restart: "always"

  postgres-post:
    image: postgres:14
    environment:
      POSTGRES_HOST: postgres
      POSTGRES_HOST_AUTH_METHOD: trust
      POSTGRES_PORT: "5432"
      POSTGRES_USER: postgres
    volumes:
      - ${PROJECT_ROOT}/bin:/blobber/bin
      # - /var/0chain/blobber/sql:/blobber/sql
    command: bash /blobber/bin/postgres-entrypoint.sh
    links:
      - postgres:postgres

  validator:
    image: 0chaindev/validator:staging
    environment:
      - DOCKER= true
    depends_on:
      - postgres-post
    links:
      - postgres-post:postgres-post
    volumes:
      - ${PROJECT_ROOT}/config:/validator/config
      - ${PROJECT_ROOT}/data:/validator/data
      - ${PROJECT_ROOT}/log:/validator/log
      - ${PROJECT_ROOT}/keys_config:/validator/keysconfig
    ports:
      - "5061:31401"
    command: ./bin/validator --port 31401 --hostname ${BLOBBER_HOST} --deployment_mode 0 --keys_file keysconfig/b0bnode02_keys.txt --log_dir /validator/log
    networks:
      default:
      testnet0:
        ipv4_address: 198.18.0.61
    restart: "always"

  blobber:
    image: 0chaindev/blobber:staging
    environment:
      DOCKER: "true"
      DB_NAME: blobber_meta
      DB_USER: blobber_user
      DB_PASSWORD: blobber
      DB_PORT: "5432"
      DB_HOST: postgres
    depends_on:
      - validator
    links:
      - validator:validator
    volumes:
      - ${PROJECT_ROOT}/config:/blobber/config
      - ${PROJECT_ROOT}/files:/blobber/files
      - ${PROJECT_ROOT}/data:/blobber/data
      - ${PROJECT_ROOT}/log:/blobber/log
      - ${PROJECT_ROOT}/keys_config:/blobber/keysconfig # keys and minio config
      - ${PROJECT_ROOT}/data/tmp:/tmp
      - ${PROJECT_ROOT}/sql:/blobber/sql
    ports:
      - "5051:5051"
      - "31501:31501"
    command: ./bin/blobber --port 5051 --grpc_port 31501 --hostname ${BLOBBER_HOST}  --deployment_mode 0 --keys_file keysconfig/b0bnode01_keys.txt --files_dir /blobber/files --log_dir /blobber/log --db_dir /blobber/data --hosturl https://${BLOBBER_HOST}
    networks:
      default:
      testnet0:
        ipv4_address: 198.18.0.91
    restart: "always"

  caddy:
    image: caddy:latest
    ports:
      - 80:80
      - 443:443
    volumes:
      - ${PROJECT_ROOT}/Caddyfile:/etc/caddy/Caddyfile
      - ${PROJECT_ROOT}/site:/srv
      - ${PROJECT_ROOT}/caddy_data:/data
      - ${PROJECT_ROOT}/caddy_config:/config
    restart: "always"

  promtail:
    image: grafana/promtail:2.3.0
    volumes:
      - ${PROJECT_ROOT}/log/:/logs
      - ${PROJECT_ROOT}/monitoringconfig/promtail-config.yaml:/mnt/config/promtail-config.yaml
    command: -config.file=/mnt/config/promtail-config.yaml
    ports:
      - "9080:9080"
    restart: "always"

  loki:
    image: grafana/loki:2.3.0
    user: "1001"
    volumes:
      - ${PROJECT_ROOT}/monitoringconfig/loki-config.yaml:/mnt/config/loki-config.yaml
    command: -config.file=/mnt/config/loki-config.yaml
    ports:
      - "3100:3100"
    restart: "always"

  prometheus:
    image: prom/prometheus
    user: root
    ports:
      - "9090:9090"
    volumes:
      - ${PROJECT_ROOT}/monitoringconfig/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
    restart: "always"
    depends_on:
    - cadvisor

  cadvisor:
    image: wywywywy/docker_stats_exporter:latest
    container_name: cadvisor
    ports:
    - 9487:9487
    volumes:
    - /var/run/docker.sock:/var/run/docker.sock
    restart: "always"

  node-exporter:
    image: prom/node-exporter:latest
    container_name: node-exporter
    restart: unless-stopped
    volumes:
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /:/rootfs:ro
    command:
      - '--path.procfs=/host/proc'
      - '--path.rootfs=/rootfs'
      - '--path.sysfs=/host/sys'
      - '--collector.filesystem.mount-points-exclude=^/(sys|proc|dev|host|etc)(\$\$|/)'
    expose:
      - 9100
    restart: "always"

  grafana:
    image: grafana/grafana:latest
    environment:
      GF_SERVER_ROOT_URL: "https://${BLOBBER_HOST}/grafana"
      GF_SECURITY_ADMIN_USER: ${GF_ADMIN_USER}
      GF_SECURITY_ADMIN_PASSWORD: ${GF_ADMIN_PASSWORD}
    volumes:
      - ${PROJECT_ROOT}/monitoringconfig/datasource.yml:/etc/grafana/provisioning/datasources/datasource.yaml
      - grafana_data:/var/lib/grafana
    ports:
      - "3040:3000"
    restart: "always"

  monitoringapi:
    image: bmanu199/monitoringapi:latest
    ports:
      - "3001:3001"
    restart: "always"

networks:
  default:
    driver: bridge
  testnet0:
    driver: bridge
    ipam:
      config:
        - subnet: 198.18.0.0/15
          gateway: 198.18.0.255

volumes:
  grafana_data:
  prometheus_data:

EOF

if [ ! -f ${PROJECT_ROOT}/keys_config/b0bnode01_keys.txt ]; then
    echo "creating keys"
    /usr/local/bin/docker-compose -f ${PROJECT_ROOT}/zchain-compose.yml up -d

    # wait for the keys keys_config/b0bnode01_keys.txt is created or not
    while [ ! -f ${PROJECT_ROOT}/keys_config/b0bnode01_keys.txt ]; do echo "wait for keys_config/b0bnode01_keys.txt"; sleep 1; done
fi

/usr/local/bin/docker-compose -f ${PROJECT_ROOT}/docker-compose.yml up -d


## setup node monitoring
