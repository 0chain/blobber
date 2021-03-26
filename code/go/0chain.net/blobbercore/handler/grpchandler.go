package handler

import (
	"context"

	"0chain.net/blobbercore/blobbergrpc"
)

type BlobberGRPCServer struct {
	storageHandler StorageHandler
	blobbergrpc.UnimplementedBlobberServer
}

func NewGRPCServer() *BlobberGRPCServer {
	return &BlobberGRPCServer{}
}

func (b *BlobberGRPCServer) GetAllocation(ctx context.Context, request *blobbergrpc.GetAllocationRequest) (*blobbergrpc.GetAllocationResponse, error) {
	ctx = setupGRPCHandlerContext(ctx, request.Context)

	allocation, err := b.storageHandler.verifyAllocation(ctx, request.Id, false)
	if err != nil {
		return nil, err
	}

	return &blobbergrpc.GetAllocationResponse{Allocation: convertAllocationToGRPCAllocation(allocation)}, nil
}
