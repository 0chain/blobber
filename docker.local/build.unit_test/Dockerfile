FROM golang:1.14.9-alpine3.12 as blobber_build

RUN apk add --update --no-cache build-base linux-headers git cmake bash perl grep

# Install Herumi's cryptography
RUN apk add gmp gmp-dev openssl-dev && \
    cd /tmp && \
    wget -O - https://github.com/herumi/mcl/archive/master.tar.gz | tar xz && \
    mv mcl* mcl && \
    make -C mcl -j $(nproc) lib/libmclbn256.so install && \
    cp mcl/lib/libmclbn256.so /usr/local/lib && \
    rm -R /tmp/mcl

RUN git clone https://github.com/herumi/bls /tmp/bls && \
    cd /tmp/bls && \
    git submodule init && \
    git submodule update && \
    make -j $(nproc) install && \
    cd - && \
    rm -R /tmp/bls

ENV SRC_DIR=/blobber
ENV GO111MODULE=on

# Download the dependencies:
# Will be cached if we don't change mod/sum files
COPY ./code/go/0chain.net/blobber/go.mod        ./code/go/0chain.net/blobber/go.sum        $SRC_DIR/go/0chain.net/blobber/
COPY ./code/go/0chain.net/blobbercore/go.mod    ./code/go/0chain.net/blobbercore/go.sum    $SRC_DIR/go/0chain.net/blobbercore/
COPY ./code/go/0chain.net/conductor/go.mod      ./code/go/0chain.net/conductor/go.sum      $SRC_DIR/go/0chain.net/conductor/
COPY ./code/go/0chain.net/core/go.mod           ./code/go/0chain.net/core/go.sum           $SRC_DIR/go/0chain.net/core/
COPY ./code/go/0chain.net/validator/go.mod        ./code/go/0chain.net/validator/go.sum        $SRC_DIR/go/0chain.net/validator/
COPY ./code/go/0chain.net/validatorcore/go.mod      ./code/go/0chain.net/validatorcore/go.sum      $SRC_DIR/go/0chain.net/validatorcore/

WORKDIR $SRC_DIR/go/0chain.net/blobber
RUN go mod download
WORKDIR $SRC_DIR/go/0chain.net/blobbercore
RUN go mod download
WORKDIR $SRC_DIR/go/0chain.net/conductor
RUN go mod download
WORKDIR $SRC_DIR/go/0chain.net/core
RUN go mod download
WORKDIR $SRC_DIR/go/0chain.net/validator
RUN go mod download
WORKDIR $SRC_DIR/go/0chain.net/validatorcore
RUN go mod download

#Add the source code
COPY ./code/go/0chain.net $SRC_DIR/go/0chain.net
WORKDIR $SRC_DIR/go
