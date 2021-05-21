package handler

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/constants"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"google.golang.org/grpc/metadata"
)

type GRPCMetaData struct {
	Client          string
	ClientKey       string
	ClientSignature string
}

func GetGRPCMetaDataFromCtx(ctx context.Context) *GRPCMetaData {
	metaData := &GRPCMetaData{}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return metaData
	}

	getMetaData := func(key string) string {
		list := md.Get(key)
		if len(list) > 0 {
			return list[0]
		}
		return ""
	}

	metaData.Client = getMetaData(common.ClientHeader)
	metaData.ClientKey = getMetaData(common.ClientKeyHeader)
	metaData.ClientSignature = getMetaData(common.ClientSignatureHeader)
	return metaData
}

func setupGRPCHandlerContext(ctx context.Context, md *GRPCMetaData, alloc string) context.Context {
	ctx = context.WithValue(ctx, constants.CLIENT_CONTEXT_KEY, md.Client)
	ctx = context.WithValue(ctx, constants.CLIENT_KEY_CONTEXT_KEY, md.ClientKey)
	ctx = context.WithValue(ctx, constants.ALLOCATION_CONTEXT_KEY, alloc)
	ctx = context.WithValue(ctx, constants.CLIENT_SIGNATURE_HEADER_KEY, md.ClientSignature)
	return ctx
}
