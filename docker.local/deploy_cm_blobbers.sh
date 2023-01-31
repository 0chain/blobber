#!/bin/bash

# setup variables
export NETWORK=demo
export DOMAIN=zus.network
export CLUSTER=dev2
export DELEGATE_WALLET="9c693cb14f29917968d6e8c909ebbea3425b4c1bc64b6732cadc2a1869f49be9"
export READ_PRICE=0.01
export WRITE_PRICE=0.1
# export MIN_STAKE=0chainminStake
# export MAX_STAKE=0chainmaxStake
# export SERVICE_CHARGE=0chainserviceCharge
export MIN_STAKE="1.0"
export MAX_STAKE="100.0"
export SERVICE_CHARGE="0.30"
export GF_ADMIN_USER=0chaingfadminuser
export GF_ADMIN_PASSWORD=0chaingfadminpassword
export PROJECT_ROOT=/var/0chain/blobber
export BLOCK_WORKER_URL=http://${NETWORK}.${DOMAIN}/dns
export EMAIL="alishahnawaz17@gmail.com"

BLOBBERCOUNT=1

#######################################################################################
#please pass the argument to check_and_install_tools to check & install package or tool.
#######################################################################################
install_tools_utilities() {
  REQUIRED_PKG=$1
  PKG_OK=$(dpkg-query -W --showformat='${Status}\n' $REQUIRED_PKG | grep "install ok installed")
  echo -e "\e[37mChecking for $REQUIRED_PKG if it is already installed. \e[73m"
  if [ "" = "$PKG_OK" ]; then
    echo -e "\e[31m  No $REQUIRED_PKG is found on the server. \e[13m\e[32m$REQUIRED_PKG installed. \e[23m \n"
    sudo apt update &> /dev/null
    sudo apt --yes install $REQUIRED_PKG &> /dev/null
  else
    echo -e "\e[32m  $REQUIRED_PKG is already installed on the server/machine.  \e[23m \n"
  fi
}

echo -e "\n \e[93m =============================================== Installing some pre-requisite tools on the server =================================================  \e[39m"
install_tools_utilities parted
install_tools_utilities build-essential
install_tools_utilities dnsutils
install_tools_utilities git
install_tools_utilities vim
install_tools_utilities jq
install_tools_utilities htop
install_tools_utilities zip
install_tools_utilities unzip

DOCKERCOMPOSEVER=v2.2.3 ; sudo apt install docker.io -y; sudo systemctl enable --now docker ; docker --version	 ; sudo curl -L "https://github.com/docker/compose/releases/download/$DOCKERCOMPOSEVER/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose &> /dev/null; sudo chmod +x /usr/local/bin/docker-compose ; docker-compose --version
sudo chmod 777 /var/run/docker.sock


#######################################################################################
#To generate binaries of zwalletcli and zboxcli
#######################################################################################
set_binaries_and_config() {
  echo -e "\n \e[93m ===================================== Creating blockworker config. ======================================  \e[39m"
  echo "---" > config.yaml
  echo "block_worker: https://${NETWORK}.${DOMAIN}/dns" >> config.yaml
  echo "signature_scheme: bls0chain" >> config.yaml
  echo "min_submit: 50" >> config.yaml
  echo "min_confirmation: 50" >> config.yaml
  echo "confirmation_chain_length: 3" >> config.yaml
  echo "max_txn_query: 5" >> config.yaml
  echo "query_sleep_time: 5" >> config.yaml

  echo -e "\n \e[93m ===================================== Downloading zwallet & zbox binaries. ======================================  \e[39m"
  wget https://github.com/0chain/zboxcli/releases/download/v1.3.11/zbox-linux.tar.gz
  tar -xvf zbox-linux.tar.gz
  rm zbox-linux.tar.gz
  wget https://github.com/0chain/zwalletcli/releases/download/v1.1.7/zwallet-linux.tar.gz
  tar -xvf zwallet-linux.tar.gz
  rm zwallet-linux.tar.gz
}
# Setting up binaries
mkdir -p ~/blobber_deploy
pushd ~/blobber_deploy
  set_binaries_and_config
popd

#######################################################################################
#To generate keys for blobber and validators.
#######################################################################################
gen_key() {
    echo -e "\n \e[93m ===================================== Creating wallet to generate key b0$2node$1_keys.json. ======================================  \e[39m"
    ./zwallet getbalance --config config.yaml --wallet b0$2node$1_keys.json --configDir . --silent
    PUBLICKEY=$( jq -r '.keys | .[] | .public_key' b0$2node$1_keys.json )
    PRIVATEKEY=$( jq -r '.keys | .[] | .private_key' b0$2node$1_keys.json )
    CLIENTID=$( jq -r .client_id b0$2node$1_keys.json )
    echo $PUBLICKEY > b0$2node$1_keys.txt
    echo $PRIVATEKEY >> b0$2node$1_keys.txt
    if [[ $2 == "b" ]] && [[ $1 -gt 0 ]]; then
      echo $3 >> b0$2node$1_keys.txt
      echo 505$1 >> b0$2node$1_keys.txt
    elif [[ $2 == "v" ]] && [[ $1 -gt 0 ]]; then
      echo $3 >> b0$2node$1_keys.txt
      echo 506$1 >> b0$2node$1_keys.txt
    fi
}
# Generating keys for blobbers
for n in $(seq 1 $BLOBBERCOUNT); do
  pushd ~/blobber_deploy
    gen_key $n b $URL $EMAIL
    gen_key $n v $URL $EMAIL
  popd
done

#######################################################################################
## blobber deployment files
#######################################################################################
get_blobber_repo() {
  # Creating directory structure for blobber deployment
  echo -e "\n \e[93m ===================================== Creating directory structure for blobber deployment. ======================================  \e[39m"

  mkdir -p ~/blobber_deploy/docker.local/bin/ ~/blobber_deploy/docker.local/keys_config/ ~/blobber_deploy/config/ ~/blobber_deploy/bin
  echo -e "\e[32mDirectory structure for blobber deployment is successfully created."

  pushd ~/blobber_deploy/

    # Install yaml query
    echo -e "\n \e[93m ===================================== Installing yq binaries. ======================================  \e[39m"
    sudo wget -qO /usr/local/bin/yq https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64
    sudo chmod a+x /usr/local/bin/yq
    yq --version

    # create Cleanup script for blobbers & validators
    echo -e "\n \e[93m ===================================== Creating cleanup script file for blobbers & validators. ======================================  \e[39m"
    wget_cleanup_script="$(wget https://raw.githubusercontent.com/0chain/blobber/staging/docker.local/bin/clean.sh -O ~/blobber_deploy/docker.local/bin/clean.sh 2>&1 | grep "HTTP" | awk '{print $6}')"
    if [[ $wget_cleanup_script == 200 ]]; then
      echo -e "\e[32m  clean.sh script file is successfully downloaded. \e[23m"
    else
      echo -e "\e[31m  Downloading clean.sh script failed. \e[13m"
      exit 1
    fi
    sudo chmod +x ~/blobber_deploy/docker.local/bin/clean.sh

    # create Init script for blobbers & validators
    echo -e "\n \e[93m ===================================== Creating Blobber Init script file for blobbers & validators. ======================================  \e[39m"
    wget_init_setup_script="$(wget https://raw.githubusercontent.com/0chain/blobber/staging/docker.local/bin/blobber.init.setup.sh -O ~/blobber_deploy/docker.local/bin/blobber.init.setup.sh 2>&1 | grep "HTTP" | awk '{print $6}')"
    if [[ $wget_init_setup_script == 200 ]]; then
      echo -e "\e[32m  blobber.init.setup.sh file is successfully downloaded. \e[23m"
    else
      echo -e "\e[31m  Downloading blobber.init.setup.sh failed. \e[13m"
      exit 1
    fi
    sudo chmod +x ~/blobber_deploy/docker.local/bin/blobber.init.setup.sh

    # create postgres entrypoint script for blobbers postgres
    echo -e "\n \e[93m ===================================== Creating postgres entrypoint script for blobbers postgres. ======================================  \e[39m"
    wget_pg_entry_script="$(wget https://raw.githubusercontent.com/0chain/blobber/staging/bin/postgres-entrypoint.sh -O ~/blobber_deploy/bin/postgres-entrypoint.sh 2>&1 | grep "HTTP" | awk '{print $6}')"
    if [[ $wget_pg_entry_script == 200 ]]; then
      echo -e "\e[32m  postgres-entrypoint.sh file is successfully downloaded. \e[23m"
    else
      echo -e "\e[31m  Downloading postgres-entrypoint.sh failed. \e[13m"
      exit 1
    fi
    sudo chmod +x ~/blobber_deploy/bin/postgres-entrypoint.sh

    # create 0chain_blobber.yaml file
    echo -e "\n \e[93m ===================================== Creating 0chain_blobber.yaml config file. ======================================  \e[39m"
    wget_blobber_yaml="$(wget https://raw.githubusercontent.com/0chain/blobber/staging/config/0chain_blobber.yaml -O ~/blobber_deploy/config/0chain_blobber.yaml 2>&1 | grep "HTTP" | awk '{print $6}')"
    if [[ $wget_blobber_yaml == 200 ]]; then
      echo -e "\e[32m  0chain_blobber.yaml file is successfully downloaded. \e[23m"
    else
      echo -e "\e[31m  Downloading 0chain_blobber.yaml failed. \e[13m"
      exit 1
    fi

    # create sc.yaml file
    echo -e "\n \e[93m ===================================== Creating 0chain_validator.yaml config file. ======================================  \e[39m"
    wget_validator_yaml="$(wget https://raw.githubusercontent.com/0chain/blobber/staging/config/0chain_validator.yaml -O ~/blobber_deploy/config/0chain_validator.yaml 2>&1 | grep "HTTP" | awk '{print $6}')"
    if [[ $wget_validator_yaml == 200 ]]; then
      echo -e "\e[32m  0chain_validator.yaml file is successfully downloaded. \e[23m"
    else
      echo -e "\e[31m  Downloading 0chain_validator.yaml failed. \e[13m"
      exit 1
    fi

    # create postgresql.conf file
    echo -e "\n \e[93m ===================================== Creating postgresql.conf config file. ======================================  \e[39m"
    wget_pg_conf="$(wget https://raw.githubusercontent.com/0chain/blobber/staging/config/postgresql.conf -O ~/blobber_deploy/config/postgresql.conf 2>&1 | grep "HTTP" | awk '{print $6}')"
    if [[ $wget_pg_conf == 200 ]]; then
      echo -e "\e[32m  postgresql.conf file is successfully downloaded. \e[23m"
    else
      echo -e "\e[31m  Downloading postgresql.conf failed. \e[13m"
      exit 1
    fi

    # create docker-compose file for blobber & validator
    echo -e "\n \e[93m ===================================== Creating docker-compose file for blobber & validator. ======================================  \e[39m"
    wget_docker_compose="$(wget https://raw.githubusercontent.com/0chain/blobber/staging/docker.local/p0docker-compose.yml -O ~/blobber_deploy/docker.local/p0docker-compose.yml 2>&1 | grep "HTTP" | awk '{print $6}')"
    if [[ $wget_docker_compose == 200 ]]; then
      echo -e "\e[32m  p0docker-compose.yml file is successfully downloaded. \e[23m"
    else
      echo -e "\e[31m  Downloading p0docker-compose.yml failed. \e[13m"
      exit 1
    fi

    # create start script for blobber & validator.
    echo -e "\n \e[93m ===================================== Creating start script file for blobber & validator. ======================================  \e[39m"
    wget_docker_compose="$(wget https://raw.githubusercontent.com/0chain/blobber/staging/docker.local/bin/p0blobber.start.sh -O ~/blobber_deploy/docker.local/bin/p0blobber.start.sh 2>&1 | grep "HTTP" | awk '{print $6}')"
    if [[ $wget_docker_compose == 200 ]]; then
      echo -e "\e[32m  p0blobber.start.sh file is successfully downloaded. \e[23m"
    else
      echo -e "\e[31m  Downloading p0blobber.start.sh failed. \e[13m"
      exit 1
    fi
    sudo chmod +x ~/blobber_deploy/docker.local/bin/p0blobber.start.sh

  popd

}

patch_configs() {
  get_blobber_repo
  pushd ~/blobber_deploy/
    DOMAINURL=${CLUSTER}.${DOMAIN}
    sed -i "s|<your-domain>|$DOMAINURL|g" ./docker.local/p0docker-compose.yml
    sed -i "s|--hostname localhost|--hostname $DOMAINURL|g" ./docker.local/p0docker-compose.yml
    yq -i '.block_worker = env(BLOCK_WORKER_URL)' ./config/0chain_blobber.yaml
    yq -i '.delegate_wallet = env(DELEGATE_WALLET)' ./config/0chain_blobber.yaml
    yq -i '.block_worker = env(BLOCK_WORKER_URL)' ./config/0chain_validator.yaml
    yq -i '.delegate_wallet = env(DELEGATE_WALLET)' ./config/0chain_validator.yaml
    sed -i "s|rate_limit: 10 |rate_limit: 100 |g" ./config/0chain_blobber.yaml
    yq -i '.read_price = env(READ_PRICE)' ./config/0chain_blobber.yaml
    yq -i '.write_price = env(WRITE_PRICE)' ./config/0chain_blobber.yaml
    yq -i '.min_stake = env(MIN_STAKE)' ./config/0chain_blobber.yaml
    yq -i '.max_stake = env(MAX_STAKE)' ./config/0chain_blobber.yaml
    yq -i '.service_charge = env(SERVICE_CHARGE)' ./config/0chain_blobber.yaml
    # yq -i '.capacity = env(CAPACITY)' ./config/0chain_blobber.yaml
    # CAPACITY=$( cat ~/cfg/blobbercap.txt ) ; if [[ $CAPACITY -lt 1073741824 ]]; then CAPACITY=107374182400 ; fi ; sed -i "s|capacity: 1073741824 #|capacity: $CAPACITY #|g" ./config/0chain_blobber.yaml
    for (( b = 1 ; b <= BLOBBERCOUNT ; b++ )) ; do 
      echo "Blobber $b" ; cp ~/blobber_deploy/b0bnode"$b"_keys.txt ~/blobber_deploy/docker.local/keys_config/b0bnode"$b"_keys.txt
      echo "Validator $b" ; cp ~/blobber_deploy/b0vnode"$b"_keys.txt ~/blobber_deploy/docker.local/keys_config/b0vnode"$b"_keys.txt
    done
	popd
}

patch_configs



##Nginx setup 
nginx_setup() {
  echo -e "\n \e[93m =============================================== Installing nginx on the server =================================================  \e[39m"
  install_tools_utilities nginx
  echo -e "\n \e[93m ============================================== Adding proxy pass to nginx config ===============================================  \e[39m"
  pushd ${HOME}/blobber_deploy/
  cat <<\EOF >${CLUSTER}.${DOMAIN}
server {
   server_name subdomain;
   add_header 'Access-Control-Expose-Headers' '*';
   location / {
       # First attempt to serve request as file, then
       # as directory, then fall back to displaying a 404.
       try_files $uri $uri/ =404;
   }
EOF
  for l in $(seq 1 $BLOBBERCOUNT)
    do
    echo "
    location /blobber0${l}/ {
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_pass http://localhost:505${l}/;
    }
    location /validator0${l}/ {
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_pass http://localhost:506${l}/;
    }" >> ./${CLUSTER}.${DOMAIN}
    done
  
  echo "}" >> ./${CLUSTER}.${DOMAIN}

  sed -i "s/subdomain/${CLUSTER}.${DOMAIN}/g" "./${CLUSTER}.${DOMAIN}"
  sudo mv ${CLUSTER}.${DOMAIN} /etc/nginx/sites-available/
  sudo ln -s /etc/nginx/sites-available/${CLUSTER}.${DOMAIN} /etc/nginx/sites-enabled/${CLUSTER}.${DOMAIN} &> /dev/null
  popd
  install_tools_utilities certbot
  install_tools_utilities python3-certbot-nginx
  echo -e "\e[37mAdding SSL to ${CLUSTER}.${DOMAIN}. \e[73m"
  sudo certbot --nginx -d ${CLUSTER}.${DOMAIN} -m $EMAIL --agree-tos -n
  # SLEEPTIME=$(awk 'BEGIN{srand(); print int(rand()*(3600+1))}'); echo "0 0,12 * * * root sleep $SLEEPTIME && certbot renew -q" | sudo tee -a /etc/crontab > /dev/null
}

nginx_setup


cd ~/blobber_deploy
./docker.local/bin/blobber.init.setup.sh
exists=$(docker network ls --filter name=testnet0 -q)
if [[ ! $exists ]] ; then
	sudo docker network create --driver=bridge --subnet=198.18.0.0/15 --gateway=198.18.0.255 testnet0
fi
cd ..


for (( b = 1 ; b <= BLOBBERCOUNT ; b++ )) ; do 
	cd ~/blobber_deploy/docker.local/blobber$b ; sudo ../bin/p0blobber.start.sh ; cd ~
done
