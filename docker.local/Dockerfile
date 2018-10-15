FROM iron/go:dev

ENV SRC_DIR=/blobber
ENV GOPATH=$SRC_DIR/go
WORKDIR $SRC_DIR

#Download the dependencies
RUN go get github.com/spf13/viper
RUN go get -u golang.org/x/crypto/...
RUN go get go.uber.org/zap
RUN go get gopkg.in/natefinch/lumberjack.v2
RUN go get github.com/gorilla/mux
RUN go get github.com/jszwec/csvutil
RUN go get github.com/dgraph-io/badger

#Add the source code
ADD ./code/go/src $SRC_DIR/go/src

RUN go build 0chain.net/blobber/blobber
