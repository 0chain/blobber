FROM golang:1.20-alpine3.18 as blobber_base

LABEL zchain="blobber"

RUN echo "https://mirrors.aliyun.com/alpine/v3.17/main" >> /etc/apk/repositories && \
    echo "https://mirrors.aliyun.com/alpine/v3.17/community" >> /etc/apk/repositories && \
    echo "https://sjc.edge.kernel.org/alpine/v3.17/main" >> /etc/apk/repositories && \
    echo "https://sjc.edge.kernel.org/alpine/v3.17/community" >> /etc/apk/repositories && \
    echo "https://uk.alpinelinux.org/alpine/v3.17/main" >> /etc/apk/repositories && \
    echo "https://uk.alpinelinux.org/alpine/v3.17/community" >> /etc/apk/repositories && \
    echo "https://dl-4.alpinelinux.org/alpine/v3.17/main" >> /etc/apk/repositories && \
    echo "https://dl-4.alpinelinux.org/alpine/v3.17/community" >> /etc/apk/repositories && \
    apk add --update --no-cache linux-headers build-base git cmake bash perl grep gmp gmp-dev

# Install Herumi's cryptography
WORKDIR /tmp


RUN apk upgrade
RUN apk del libstdc++ gmp-dev openssl-dev vips-dev
RUN apk add --update --no-cache --repository http://dl-cdn.alpinelinux.org/alpine/edge/main libstdc++ gmp-dev openssl-dev vips-dev
