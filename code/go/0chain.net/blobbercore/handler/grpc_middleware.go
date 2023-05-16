package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/improbable-eng/grpc-web/go/grpcweb"

	"github.com/gorilla/mux"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
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
		deadline := time.Now().Add(TIMEOUT_SECONDS * time.Second)
		ctx, canceler := context.WithDeadline(ctx, deadline)
		defer canceler()

		return handler(ctx, req)
	}
}

func NewGRPCServerWithMiddlewares(r *mux.Router) *grpc.Server {
	srv := grpc.NewServer(
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

	registerGRPCServices(r, srv)

	// adds grpc-web middleware
	wrappedServer := grpcweb.WrapServer(srv)
	r.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					escapedUrl := sanitizeString(r.URL.String())
					logging.Logger.Error("[recover]grpc", zap.String("url", escapedUrl), zap.Any("err", err))
				}
			}()
			if wrappedServer.IsGrpcWebRequest(r) {
				wrappedServer.ServeHTTP(w, r)
				return
			}
			h.ServeHTTP(w, r)
		})
	})

	return srv
}
