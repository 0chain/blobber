package handler

import (
	"context"
	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/convert"
	"github.com/pkg/errors"
)

func (b *blobberGRPCService) UpdateObjectAttributes(ctx context.Context, request *blobbergrpc.UpdateObjectAttributesRequest) (*blobbergrpc.UpdateObjectAttributesResponse, error) {

	ctx = setupGrpcHandlerContext(ctx, getGRPCMetaDataFromCtx(ctx))
	response, err := storageHandler.UpdateObjectAttributes(ctx, request)

	if err != nil {
		return nil, errors.Wrap(err, "failed to UpdateObjectAttributes")
	}

	return response, nil
}

func (b *blobberGRPCService) CopyObject(ctx context.Context, request *blobbergrpc.CopyObjectRequest) (*blobbergrpc.CopyObjectResponse, error) {

	ctx = setupGrpcHandlerContext(ctx, getGRPCMetaDataFromCtx(ctx))
	response, err := storageHandler.CopyObject(ctx, request)

	if err != nil {
		return nil, errors.Wrap(err, "failed to CopyObject")
	}

	return response, nil
}

func (b *blobberGRPCService) RenameObject(ctx context.Context, request *blobbergrpc.RenameObjectRequest) (*blobbergrpc.RenameObjectResponse, error) {
	ctx = setupGrpcHandlerContext(ctx, getGRPCMetaDataFromCtx(ctx))

	response, err := storageHandler.RenameObject(ctx, request)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (b *blobberGRPCService) DownloadFile(ctx context.Context, req *blobbergrpc.DownloadFileRequest) (*blobbergrpc.DownloadFileResponse, error) {
	r, err := convert.DownloadFileGRPCToHTTP(req)
	if err != nil {
		return nil, err
	}

	httpRequestWithMetaData(r, getGRPCMetaDataFromCtx(ctx), req.Allocation)

	resp, err := DownloadHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.DownloadFileResponseCreator(resp), nil
}

func (b *blobberGRPCService) UploadFile(ctx context.Context, req *blobbergrpc.UploadFileRequest) (*blobbergrpc.UploadFileResponse, error) {

	r, err := convert.WriteFileGRPCToHTTP(req)
	if err != nil {
		return nil, err
	}

	httpRequestWithMetaData(r, getGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path":          {req.Path},
		"connection_id": {req.ConnectionId},
		"uploadMeta":    {req.UploadMeta},
		"updateMeta":    {req.UpdateMeta},
	}

	resp, err := UploadHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.UploadFileResponseCreator(resp), nil
}
