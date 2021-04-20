package handler

import (
	"context"
	"time"

	"0chain.net/core/common"
	"0chain.net/core/logging"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const (
	TIMEOUT_SECONDS = 10 // to set deadline for requests
)

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

		err = GetMetaDataStore().GetTransaction(ctx).Commit().Error
		if err != nil {
			return nil, common.NewErrorf("commit_error",
				"error committing to meta store: %v", err)
		}

		return resp, err
	}
}

func unaryTimeoutInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		deadline := time.Now().Add(time.Duration(TIMEOUT_SECONDS * time.Second))
		ctx, _ = context.WithDeadline(ctx, deadline)
		return handler(ctx, req)
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
			unaryTimeoutInterceptor(), // should always be the lastest, to be "innermost"
		),
	)
}
