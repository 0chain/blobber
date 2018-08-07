FROM iron/go:dev

WORKDIR /blobber

ENV SRC_DIR=/blobber

ADD . $SRC_DIR/

ENV GOPATH=/blobber/code/go


#Download the dependencies
RUN go get github.com/spf13/viper
RUN go get -u golang.org/x/crypto/...
RUN go get go.uber.org/zap
RUN go get gopkg.in/natefinch/lumberjack.v2
RUN go get github.com/gorilla/mux

RUN cd code/go
RUN go build 0chain.net/blobber/blobber

