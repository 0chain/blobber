# GRPC Migration

Modify the '.proto' file in `blobbergrpc/proto/blobber.proto` and run 
`scripts/generate-grpc.sh` to add new api's.

GRPC API is implemented in `handler/grpc_handler.go`.

## Plugins
* [grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway) 
plugin is being used to expose a REST api for grpc incompatible clients.