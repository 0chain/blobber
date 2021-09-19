FROM blobber_base as blobber_build

LABEL zchain="blobber"


ENV SRC_DIR=/0chain
ENV GO111MODULE=on
#ENV GOPROXY=https://goproxy.cn,direct 

# Download the dependencies:
# Will be cached if we don't change mod/sum files
COPY .  $SRC_DIR
# COPY ./gosdk  /gosdk

RUN cd $SRC_DIR/ && go mod download 


WORKDIR $SRC_DIR/code/go/0chain.net/blobber

ARG GIT_COMMIT
ENV GIT_COMMIT=$GIT_COMMIT
RUN go build -v -tags "bn256 development" -ldflags "-X 0chain.net/core/build.BuildTag=$GIT_COMMIT"

# Copy the build artifact into a minimal runtime image:
FROM golang:1.17.1-alpine3.14
RUN apk add gmp gmp-dev openssl-dev git
COPY --from=blobber_build  /usr/local/lib/libmcl*.so \
                        /usr/local/lib/libbls*.so \
                        /usr/local/lib/


ENV APP_DIR=/blobber
WORKDIR $APP_DIR
COPY --from=blobber_build /0chain/code/go/0chain.net/blobber/blobber $APP_DIR/bin/blobber

