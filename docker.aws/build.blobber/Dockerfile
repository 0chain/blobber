#FROM ubuntu:16.04
#RUN groupadd -r 0chain && useradd --no-log-init -r -g 0chain 0chain

FROM golang:1.11.4-alpine3.8 as blobber_build

ENV SRC_DIR=/0chain
ENV GOPATH=$GOPATH:$SRC_DIR/go
WORKDIR $SRC_DIR

RUN apk add --update --no-cache build-base linux-headers git cmake bash perl grep

#Download the dependencies
RUN go get github.com/spf13/viper
RUN go get -u golang.org/x/crypto/...
RUN go get go.uber.org/zap
RUN go get gopkg.in/natefinch/lumberjack.v2
RUN go get github.com/gorilla/mux
RUN go get github.com/dgraph-io/badger
RUN go get github.com/gorilla/handlers

#Add the source code
ADD ./code/go/src $SRC_DIR/go/src

RUN go build 0chain.net/blobber/blobber

FROM alpine:3.8
ENV APP_DIR=/0chain
WORKDIR $APP_DIR
COPY --from=blobber_build $APP_DIR/go/0chain.net/blobber/blobber/blobber $APP_DIR/bin/blobber

#Store all files and run environment under 0chain userid.
RUN addgroup -g 2000 -S 0chain && adduser -u 2000 -S 0chain -G 0chain
USER 0chain:0chain
