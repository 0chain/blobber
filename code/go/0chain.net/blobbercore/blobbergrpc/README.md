# GRPC Endpoints

We are using Buf, which is building a better way to work with Protocol Buffers. Buf supports
linting & detecting breaking changes for our gRPC protos as well as generating the gRPC stubs.

### Pre-Requisites
- Setup [Buf](https://buf.build) tool on local, run
```sh
make deps
```
- Installation of required go-deps for gRPC, run
```sh
go install \
github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 \
google.golang.org/protobuf/cmd/protoc-gen-go \
google.golang.org/grpc/cmd/protoc-gen-go-grpc
```

## Development

- All the protos present in the project can be found by `buf ls-files`.

- Generation of stubs on local can be done by ` buf generate -o ./code/go/0chain.net/blobbercore/blobbergrpc/` 

- Buf is also capable of linting the protos according to the best practices of the gRPC `buf lint`

## Testing

The current grpc implementation supports server reflection in development environment.
You can interact with the api using https://github.com/gusaul/grpcox. While running locally make sure
to use docker network ip and not localhost.

Make sure the server is running on `--deployment_mode 0` to use server reflection.

## Documentation

The basic documentation can be found here - https://grpc.io/docs/languages/go/basics/.

The advanced documentation can be found here - https://github.com/grpc/grpc-go/tree/master/Documentation.


