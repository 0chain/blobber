FROM blobber_base as blobber_build

RUN apk add --update --no-cache build-base linux-headers git cmake bash perl grep

ENV SRC_DIR=/blobber
ENV GO111MODULE=on

# Download the dependencies:
# Will be cached if we don't change mod/sum files
COPY ./go.mod          ./go.sum          $SRC_DIR/
RUN cd $SRC_DIR && go mod download

#Add the source code
ADD ./goose          $SRC_DIR/goose
ADD ./code/go/0chain.net $SRC_DIR/code/go/0chain.net

WORKDIR $SRC_DIR/code/go/0chain.net/blobber

RUN CGO_ENABLED=1 go build -v -tags "bn256 development integration_tests" -ldflags "-X github.com/0chain/blobber/code/go/0chain.net/core/build.BuildTag=$GIT_COMMIT"

# Copy the build artifact into a minimal runtime image:
FROM golang:1.19-alpine3.17
RUN apk add gmp gmp-dev openssl-dev
COPY --from=blobber_build  /usr/local/lib/libmcl*.so \
                        /usr/local/lib/libbls*.so \
                        /usr/local/lib/
ENV APP_DIR=/blobber
WORKDIR $APP_DIR
COPY --from=blobber_build $APP_DIR/code/go/0chain.net/blobber/blobber $APP_DIR/bin/blobber