FROM golang:1.18.1-alpine3.15 as blobber_base

LABEL zchain="blobber"

# https://mirrors.alpinelinux.org/
# RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

RUN echo "https://mirrors.aliyun.com/alpine/v3.14/main" >> /etc/apk/repositories && \
    echo "https://mirrors.aliyun.com/alpine/v3.14/community" >> /etc/apk/repositories && \
    echo "https://sjc.edge.kernel.org/alpine/v3.14/main" >> /etc/apk/repositories && \
    echo "https://sjc.edge.kernel.org/alpine/v3.14/community" >> /etc/apk/repositories && \
    echo "https://uk.alpinelinux.org/alpine/v3.14/main" >> /etc/apk/repositories && \
    echo "https://uk.alpinelinux.org/alpine/v3.14/community" >> /etc/apk/repositories && \
    echo "https://dl-4.alpinelinux.org/alpine/v3.14/main" >> /etc/apk/repositories && \
    echo "https://dl-4.alpinelinux.org/alpine/v3.14/community" >> /etc/apk/repositories && \
    apk add --update --no-cache build-base linux-headers git cmake bash perl grep gmp gmp-dev openssl-dev 

# Install Herumi's cryptography
WORKDIR /tmp

COPY ./docker.local/bin/mcl.tar.gz ./docker.local/bin/bls.tar.gz ./

RUN tar zxvf mcl.tar.gz && rm mcl.tar.gz && mv mcl* mcl && \
    tar zxvf bls.tar.gz && rm bls.tar.gz && mv bls* bls && \
    make -C mcl -j $(nproc) lib/libmclbn256.so install && \
    cp mcl/lib/libmclbn256.so /usr/local/lib && \
    make MCL_DIR=$(pwd)/mcl -C bls -j $(nproc) install && \
    rm -R /tmp/mcl && rm -R /tmp/bls

#ENV GOPROXY=https://goproxy.cn
