#!/bin/bash

root=$(pwd)
hostname=`ifconfig | grep "inet " | grep -Fv 127.0.0.1 | grep broadcast | awk '{print $2}'`


ips=`ifconfig | grep "inet " | grep 198.18.0 | wc -l`


#fix docker network issue for Mac OS X platform
if [ "$(uname)" == "Darwin" ] && [ $ips != 31 ]
then
    # 0dns
    sudo ifconfig lo0 alias 198.18.0.98
    # sharders
    sudo ifconfig lo0 alias 198.18.0.81
    sudo ifconfig lo0 alias 198.18.0.82
    sudo ifconfig lo0 alias 198.18.0.83
    sudo ifconfig lo0 alias 198.18.0.84
    sudo ifconfig lo0 alias 198.18.0.85
    sudo ifconfig lo0 alias 198.18.0.86
    sudo ifconfig lo0 alias 198.18.0.87
    sudo ifconfig lo0 alias 198.18.0.88
    # miners
    sudo ifconfig lo0 alias 198.18.0.71
    sudo ifconfig lo0 alias 198.18.0.72
    sudo ifconfig lo0 alias 198.18.0.73
    sudo ifconfig lo0 alias 198.18.0.74
    sudo ifconfig lo0 alias 198.18.0.75
    sudo ifconfig lo0 alias 198.18.0.76
    sudo ifconfig lo0 alias 198.18.0.77
    sudo ifconfig lo0 alias 198.18.0.78
    # blobbers
    sudo ifconfig lo0 alias 198.18.0.91
    sudo ifconfig lo0 alias 198.18.0.92
    sudo ifconfig lo0 alias 198.18.0.93
    sudo ifconfig lo0 alias 198.18.0.94
    sudo ifconfig lo0 alias 198.18.0.95
    sudo ifconfig lo0 alias 198.18.0.96
    sudo ifconfig lo0 alias 198.18.0.97
    # validators
    sudo ifconfig lo0 alias 198.18.0.61
    sudo ifconfig lo0 alias 198.18.0.62
    sudo ifconfig lo0 alias 198.18.0.63
    sudo ifconfig lo0 alias 198.18.0.64
    sudo ifconfig lo0 alias 198.18.0.65
    sudo ifconfig lo0 alias 198.18.0.66
    sudo ifconfig lo0 alias 198.18.0.67
fi


echo "
**********************************************
  Welcome to blobber/validator development CLI 
**********************************************

"

echo "Hostname: $hostname"

set_hostname() {

    read -p "change hostname($hostname), please enter your hostname: " hostname
    echo ""
    echo "> hostname is updated to: $hostname"
}

change_zcn() {
    zcn=$(cat ../config/0chain_blobber.yaml | grep '^block_worker' | awk -F ' ' '{print $2}')
    read -p "change zcn($zcn), please enter your zcn: " yourZCN

    find ../config/ -name "0chain_blobber.yaml" -exec sed -i '' "s/block_worker/#block_worker/g" {} \;
    echo "block_worker: $yourZCN" >> ../config/0chain_blobber.yaml
    zcn=$(cat ../config/0chain_blobber.yaml | grep '^block_worker' | awk -F ' ' '{print $2}')
    echo "> zcn is updated to: $zcn"
}

install_debuggger() {
    [ -d ../.vscode ] || mkdir -p ../.vscode
    sed "s/Hostname/$hostname/g" launch.json > ../.vscode/launch.json
    echo "debugbbers are installed"
}

cleanAll() {
    
    cd $root
    rm -rf ./data && echo "data is removed"
}


echo " "
echo "Please select which blobber/validator you will work on: "

select i in "1" "2" "3" "clean all" "install debugers on .vscode/launch.json" "set hostname" "change zcn"; do
    case $i in
        "1"             ) break;;
        "2"             ) break;;
        "3"             ) break;;
        "clean all"     ) cleanAll ;;
        "install debugers on .vscode/launch.json" ) install_debuggger;;
        "set hostname"  ) set_hostname;;
        "change zcn"  ) change_zcn;;
    esac
done


install_postgres () {

    echo Installing blobber_postgres in docker...

    [ ! "$(docker ps -a | grep blobber_postgres)" ] && docker run --name blobber_postgres --restart always -p 5432:5432 -e POSTGRES_PASSWORD=postgres -d postgres:11

    [ -d "./data/blobber$i" ] || mkdir -p "./data/blobber$i" 

    echo Initializing database

    [ -d "./data/blobber$i/sql" ] && rm -rf  [ -d "./data/blobber$i/sql" ]

    cp -r ../sql "./data/blobber$i/"
    cd "./data/blobber$i/sql"

    find . -name "*.sql" -exec sed -i '' "s/blobber_user/blobber_user$i/g" {} \;
    find . -name "*.sql" -exec sed -i '' "s/blobber_meta/blobber_meta$i/g" {} \;


    cd $root
    [ -d "./data/blobber$i/bin" ] && rm -rf  [ -d "./data/blobber$i/bin" ]
    cp -r ../bin "./data/blobber$i/"


    cd $root
  
    [ ! "docker ps -a | grep blobber_postgres_init" ] && docker rm blobber_postgres_init --force


    docker run --name blobber_postgres_init \
    --link blobber_postgres:postgres \
    -e  POSTGRES_PORT=5432 \
    -e  POSTGRES_HOST=postgres \
    -e  POSTGRES_USER=postgres  \
    -e  POSTGRES_PASSWORD=postgres \
    -v  $root/data/blobber$i/bin:/blobber/bin \
    -v  $root/data/blobber$i/sql:/blobber/sql \
    postgres:11 bash /blobber/bin/postgres-entrypoint.sh 

    docker rm blobber_postgres_init --force

}

prepareRuntime() {
    echo "Prepare blobber $i: config,files, data, log .."
    cd $root
    [ -d ./data/blobber$i/config ] && rm -rf $root/data/blobber$i/config
    cp -r ../config "./data/blobber$i/"

    cd  ./data/blobber$i/config/

    find . -name "*.yaml" -exec sed -i '' "s/blobber_user/blobber_user$i/g" {} \;
    find . -name "*.yaml" -exec sed -i '' "s/blobber_meta/blobber_meta$i/g" {} \;
    find . -name "*.yaml" -exec sed -i '' "s/postgres/127.0.0.1/g" {} \;
    cd $root/data/blobber$i/

    [ -d files ] || mkdir files
    [ -d data ] || mkdir data
    [ -d log ] || mkdir log
}

start_blobber () {
    echo "Building blobber $i ..."
    cd ../code/go/0chain.net/blobber   
    go build -v -tags "bn256 development" -gcflags="-N -l" -ldflags "-X 0chain.net/core/build.BuildTag=dev" -o $root/data/blobber$i/blobber .

    prepareRuntime;

    echo "Starting blobber $i ..."

    cd $root
    port="505$i"
    grpc_port="703$i"

    keys_file="../docker.local/keys_config/b0bnode${i}_keys.txt"
    minio_file="../docker.local/keys_config/minio_config.txt"
    config_dir="./data/blobber$i/config"
    files_dir="./data/blobber$i/files"
    log_dir="./data/blobber$i/log"
    db_dir="./data/blobber$i/data"

    ./data/blobber$i/blobber --port $port --grpc_port $grpc_port -hostname $hostname --deployment_mode 0 --keys_file $keys_file  --files_dir $files_dir --log_dir $log_dir --db_dir $db_dir  --minio_file $minio_file --config_dir $config_dir
}

start_validator () {
    echo "Building validator $i ..."

    cd ../code/go/0chain.net/validator   
    go build -v -tags "bn256 development" -gcflags="-N -l" -ldflags "-X 0chain.net/core/build.BuildTag=dev" -o $root/data/blobber$i/validator .


    prepareRuntime;

    echo "Starting validator $i ..."


    cd $root
    port="506$i"
    hostname="localhost"
    keys_file="../docker.local/keys_config/b0bnode${i}_keys.txt"
    config_dir="./data/blobber$i/config"
    log_dir="./data/blobber$i/log"

    ./data/blobber$i/validator --port $port -hostname $hostname --deployment_mode 0 --keys_file $keys_file  --log_dir $log_dir --config_dir $config_dir
}

clean () {
    echo "Building blobber $i"

    cd $root

    rm -rf "./data/blobber$i"
}


echo "
**********************************************
            Blobber/Validator $i
**********************************************"

echo " "
echo "Please select what you will do: "

select f in "install postgres" "start blobber" "start validator" "clean"; do
    case $f in
        "install postgres"  )   install_postgres;     break;;
        "start blobber"     )   start_blobber;        break;;
        "start validator"   )   start_validator;      break;;
        "clean"             )   clean;      break;;
    esac
done


