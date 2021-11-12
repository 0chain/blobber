FROM golang:1.17.1-alpine3.14 as blobber_base

LABEL zchain="blobber"

# RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

RUN echo "https://mirrors.aliyun.com/alpine/v3.14/main" >> /etc/apk/repositories
RUN echo "https://mirrors.aliyun.com/alpine/v3.14/community" >> /etc/apk/repositories

RUN echo "https://sjc.edge.kernel.org/alpine/v3.14/main" >> /etc/apk/repositories
RUN echo "https://sjc.edge.kernel.org/alpine/v3.14/community" >> /etc/apk/repositories

RUN echo "https://uk.alpinelinux.org/alpine/v3.14/main" >> /etc/apk/repositories
RUN echo "https://uk.alpinelinux.org/alpine/v3.14/community" >> /etc/apk/repositories

RUN echo "https://dl-4.alpinelinux.org/alpine/v3.14/main" >> /etc/apk/repositories
RUN echo "https://dl-4.alpinelinux.org/alpine/v3.14/community" >> /etc/apk/repositories

RUN echo "https://mirror.yandex.ru/alpine/v3.14/main" >> /etc/apk/repositories
RUN echo "https://mirror.yandex.ru/alpine/v3.14/community" >> /etc/apk/repositories


RUN apk add --update --no-cache build-base linux-headers git cmake bash perl grep

# Install Herumi's cryptography
RUN apk add gmp gmp-dev openssl-dev && \
    cd /tmp && \
    wget -O - https://github.com/herumi/mcl/archive/master.tar.gz | tar xz && \
    wget -O - https://github.com/herumi/bls/archive/master.tar.gz | tar xz && \
    mv mcl* mcl && \
    mv bls* bls && \
    make -C mcl -j $(nproc) lib/libmclbn256.so install && \
    cp mcl/lib/libmclbn256.so /usr/local/lib && \
    make MCL_DIR=$(pwd)/mcl -C bls -j $(nproc) install && \
    rm -R /tmp/mcl && \
    rm -R /tmp/bls
