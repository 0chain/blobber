# Blobber on ec2 / vm / bare metal over https


## Prerequisite

- ec2 / vm / bare metal with docker installed

## Initial Setup

  

### Directory Setup for Blobbers

  

In the git/blobber run the following command

  

```

$ ./docker.local/bin/blobber.init.setup.sh

```

  

## Building and Starting the Nodes

  
1. Setup a network called testnet0 for each of these node containers to talk to each other.
 
 ```
$ docker network create --driver=bridge --subnet=198.18.0.0/15 --gateway=198.18.0.255 testnet0
```
2. Update block_worker url you give in the `config/0chain_blobber/validator.yaml` and `config/0chain_blobber/0chain_blobber.yaml` config file.

For example: If you want to connect to five network, set


```
block_worker: https://five.devnet-0chain.net/dns
```

3. Edit `docker.local/p0docker-compose.yml` and replace `< localhost >` in command with your domain. 
  

4. Go to blobber1 directory (git/blobber/docker.local/blobber1) and run the container using:

  

```

$ ../bin/p0blobber.start.sh


```

This will join blobber1 to the network. You can repeat step 4 for other blobbers to join them to the network.

## Configuring Https

Note: You can skip this step if you have nginx/haproxy already setup with ssl in place. Just add paths in the config file and restart the service. 

1. Go to https directory in blobber repo.
```
cd /blobber/https
```

2. Edit docker-compose.yml and replace <your_email>, <your_domain> with your email and domain. Make sure to add 'A' type record for your domain and ip address with your domain provider.


3. Deploy nginx and certbot using the following command
```
docker-compose up -d
```

4. Check certbot logs and see if certificate is generated. You will find "Congratulations! Your certificate and chain have been saved at: /etc/letsencrypt/live/<your_domain>/fullchain.pem" in the logs if the certificate is generated properly.

```
docker logs -f https_certbot_1 
```

4. Edit /conf.d/nginx.conf to uncomment required locations in config for port 80. Uncomment all lines in server config for port 443 and comment locations which are not required. Don't forget to reploce <your_domain> with your domain. 

5. Restart docker compose and you will be able to access blobbers over https.

```
docker-compose restart
```
