# GRPC Migration

Modify the '.proto' file in `blobbergrpc/proto/blobber.proto` and run 
`scripts/generate-grpc.sh` to add new api's.

GRPC API is implemented in `handler/grpc_handler.go`.

## Plugins
* [grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway) 
plugin is being used to expose a REST api for grpc incompatible clients.

## Interacting with the api
The current grpc implementation supports server reflection in development environment.
You can interact with the api using https://github.com/gusaul/grpcox.

Make sure the server is running on `--deployment_mode 0` to use server reflection.