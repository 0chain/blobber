FROM golang:1.17.1-alpine3.14 as blobber_base

LABEL zchain="blobber"

# https://mirrors.alpinelinux.org/
# RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

RUN echo "https://mirrors.aliyun.com/alpine/v3.14/main" >> /etc/apk/repositories
RUN echo "https://mirrors.aliyun.com/alpine/v3.14/community" >> /etc/apk/repositories

RUN echo "https://sjc.edge.kernel.org/alpine/v3.14/main" >> /etc/apk/repositories
RUN echo "https://sjc.edge.kernel.org/alpine/v3.14/community" >> /etc/apk/repositories

# RUN echo "https://uk.alpinelinux.org/alpine/v3.14/main" >> /etc/apk/repositories
# RUN echo "https://uk.alpinelinux.org/alpine/v3.14/community" >> /etc/apk/repositories

# RUN echo "https://dl-4.alpinelinux.org/alpine/v3.14/main" >> /etc/apk/repositories
# RUN echo "https://dl-4.alpinelinux.org/alpine/v3.14/community" >> /etc/apk/repositories



RUN apk add --update --no-cache build-base linux-headers git cmake bash perl grep

# Install Herumi's cryptography
RUN apk add gmp gmp-dev openssl-dev 

WORKDIR /tmp
RUN wget https://github.com/herumi/mcl/archive/master.tar.gz
RUN tar zxvf master.tar.gz &&  mv mcl* mcl

RUN wget  https://github.com/herumi/bls/archive/master.tar.gz 
RUN tar zxvf master.tar.gz && && mv bls* bls  

RUN make -C mcl -j $(nproc) lib/libmclbn256.so install 
RUN cp mcl/lib/libmclbn256.so /usr/local/lib 

RUN make MCL_DIR=$(pwd)/mcl -C bls -j $(nproc) install 

RUN rm -R /tmp/mcl && rm -R /tmp/bls
