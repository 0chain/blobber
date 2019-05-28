#FROM ubuntu:16.04
#RUN groupadd -r 0chain && useradd --no-log-init -r -g 0chain 0chain

FROM golang:1.11.4-alpine3.8 as blobber_build

RUN apk add --update --no-cache build-base linux-headers git cmake bash perl grep

ENV SRC_DIR=/0chain
ENV GO111MODULE=on

# Download the dependencies:
# Will be cached if we don't change mod/sum files
COPY ./code/go/0chain.net/core/go.mod          ./code/go/0chain.net/core/go.sum          $SRC_DIR/go/0chain.net/core/
COPY ./code/go/0chain.net/blobbercore/go.mod     ./code/go/0chain.net/blobbercore/go.sum     $SRC_DIR/go/0chain.net/blobbercore/
COPY ./code/go/0chain.net/blobber/go.mod     ./code/go/0chain.net/blobber/go.sum     $SRC_DIR/go/0chain.net/blobber/

WORKDIR $SRC_DIR/go/0chain.net/blobber
RUN go mod download


#Add the source code
ADD ./code/go/0chain.net $SRC_DIR/go/0chain.net

RUN go build -v

# Copy the build artifact into a minimal runtime image:
FROM alpine:3.8
ENV APP_DIR=/0chain
WORKDIR $APP_DIR
COPY --from=blobber_build $APP_DIR/go/0chain.net/blobber/blobber $APP_DIR/bin/blobber

#Store all files and run environment under 0chain userid.
RUN addgroup -g 2000 -S 0chain && adduser -u 2000 -S 0chain -G 0chain
USER 0chain:0chain
