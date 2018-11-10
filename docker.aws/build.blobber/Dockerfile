#Store all files and run environment under 0chain userid.
FROM ubuntu:16.04
RUN groupadd -r 0chain && useradd --no-log-init -r -g 0chain 0chain

FROM iron/go:dev

ENV SRC_DIR=/blobber
ENV GOPATH=$GOPATH:$SRC_DIR/go
WORKDIR $SRC_DIR
ADD ./docker.aws/build.blobber/BLOBBER_VERSION .

#Download the dependencies
RUN go get github.com/spf13/viper
RUN go get -u golang.org/x/crypto/...
RUN go get go.uber.org/zap
RUN go get gopkg.in/natefinch/lumberjack.v2
RUN go get github.com/gorilla/mux
RUN go get github.com/jszwec/csvutil
RUN go get github.com/dgraph-io/badger
RUN go get github.com/gorilla/handlers

#Add the source code
ADD ./code/go/src $SRC_DIR/go/src

RUN go build 0chain.net/blobber/blobber
WORKDIR $SRC_DIR/bin
RUN cp $SRC_DIR/blobber $SRC_DIR/bin

#Run the blobber
WORKDIR $SRC_DIR
USER 2000:2000
