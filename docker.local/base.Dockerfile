FROM golang:1.21-alpine3.18 as blobber_base

LABEL zchain="blobber"

RUN  apk add --update --no-cache linux-headers build-base git cmake bash perl grep 

# Install Herumi's cryptography
WORKDIR /tmp

RUN apk upgrade
RUN apk del libstdc++ gmp-dev openssl-dev vips-dev
RUN apk add --update --no-cache --repository http://dl-cdn.alpinelinux.org/alpine/edge/main libstdc++ gmp-dev openssl-dev vips-dev