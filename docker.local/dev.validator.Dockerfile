FROM blobber_base as validator_build

LABEL zchain="validator"


ENV SRC_DIR=/blobber
ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct 


# Download the dependencies:
# Will be cached if we don't change mod/sum files
COPY ./code/go/0chain.net/go.mod          ./code/go/0chain.net/go.sum          $SRC_DIR/go/0chain.net/

COPY ./gosdk  /gosdk

RUN cd $SRC_DIR/go/0chain.net && go mod download -x

#Add the source code
ADD ./code/go/0chain.net $SRC_DIR/go/0chain.net


WORKDIR $SRC_DIR/go/0chain.net/validator

RUN go build -v -tags "bn256 development" -ldflags "-X 0chain.net/core/build.BuildTag=$GIT_COMMIT"

# Copy the build artifact into a minimal runtime image:
FROM golang:1.11.4-alpine3.8
RUN apk add gmp gmp-dev openssl-dev
COPY --from=validator_build  /usr/local/lib/libmcl*.so \
                        /usr/local/lib/libbls*.so \
                        /usr/local/lib/
ENV APP_DIR=/blobber
WORKDIR $APP_DIR
COPY --from=validator_build $APP_DIR/go/0chain.net/validator/validator $APP_DIR/bin/validator
