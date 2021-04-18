package handler

import (
	"context"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"

	"0chain.net/core/common"

	"go.uber.org/zap"

	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"

	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"

	"github.com/gorilla/mux"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	"google.golang.org/grpc"

	"0chain.net/blobbercore/blobbergrpc"
	"0chain.net/core/logging"
)

type blobberGRPCService struct {
	storageHandler StorageHandler
	blobbergrpc.UnimplementedBlobberServer
}

func unaryDatabaseTransactionInjector() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		logger := ctxzap.Extract(ctx)

		ctx = GetMetaDataStore().CreateTransaction(ctx)
		resp, err := handler(ctx, req)
		if err != nil {
			var rollErr = GetMetaDataStore().GetTransaction(ctx).Rollback().Error
			if rollErr != nil {
				logger.Error("couldn't rollback", zap.Error(err))
			}
			return nil, err
		}

		err = GetMetaDataStore().GetTransaction(ctx).Commit().Error()
		if err != nil {
			return nil, common.NewErrorf("commit_error",
				"error committing to meta store: %v", err)
		}

		return resp, err
	}
}

func NewServerWithMiddlewares() *grpc.Server {
	return grpc.NewServer(
		grpc.ChainStreamInterceptor(
			grpc_zap.StreamServerInterceptor(logging.Logger),
			grpc_recovery.StreamServerInterceptor(),
		),
		grpc.ChainUnaryInterceptor(
			grpc_zap.UnaryServerInterceptor(logging.Logger),
			grpc_recovery.UnaryServerInterceptor(),
			unaryDatabaseTransactionInjector(),
		),
	)
}

func RegisterGRPCServices(r *mux.Router, server *grpc.Server) {
	blobberService := newGRPCBlobberService()
	mux := runtime.NewServeMux()
	blobbergrpc.RegisterBlobberServer(server, blobberService)
	blobbergrpc.RegisterBlobberHandlerServer(context.Background(), mux, blobberService)
	r.PathPrefix("/v2").Handler(mux)
}

func newGRPCBlobberService() *blobberGRPCService {
	return &blobberGRPCService{}
}

func (b *blobberGRPCService) GetAllocation(ctx context.Context, request *blobbergrpc.GetAllocationRequest) (*blobbergrpc.GetAllocationResponse, error) {
	ctx = setupGRPCHandlerContext(ctx, request.Context)

	allocation, err := b.storageHandler.verifyAllocation(ctx, request.Id, false)
	if err != nil {
		return nil, err
	}

	return &blobbergrpc.GetAllocationResponse{Allocation: convertAllocationToGRPCAllocation(allocation)}, nil
}
