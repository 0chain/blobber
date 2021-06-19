package handler

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/convert"
	"net/http"
)

func (b *blobberGRPCService) UpdateObjectAttributes(ctx context.Context, req *blobbergrpc.UpdateObjectAttributesRequest) (*blobbergrpc.UpdateObjectAttributesResponse, error) {
	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, GetGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path":          {req.Path},
		"path_hash":     {req.PathHash},
		"connection_id": {req.ConnectionId},
		"attributes":    {req.Attributes},
	}

	resp, err := UpdateAttributesHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.UpdateObjectAttributesResponseCreator(resp), nil
}

func (b *blobberGRPCService) CopyObject(ctx context.Context, req *blobbergrpc.CopyObjectRequest) (*blobbergrpc.CopyObjectResponse, error) {
	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, GetGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path":          {req.Path},
		"path_hash":     {req.PathHash},
		"connection_id": {req.ConnectionId},
		"dest":          {req.Dest},
	}

	resp, err := CopyHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.CopyObjectResponseCreator(resp), nil
}

func (b *blobberGRPCService) RenameObject(ctx context.Context, req *blobbergrpc.RenameObjectRequest) (*blobbergrpc.RenameObjectResponse, error) {
	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, GetGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path":          {req.Path},
		"path_hash":     {req.PathHash},
		"connection_id": {req.ConnectionId},
		"new_name":      {req.NewName},
	}

	resp, err := RenameHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.RenameObjectResponseCreator(resp), nil
}

func (b *blobberGRPCService) DownloadFile(ctx context.Context, req *blobbergrpc.DownloadFileRequest) (*blobbergrpc.DownloadFileResponse, error) {

	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		return nil, err
	}

	httpRequestWithMetaData(r, GetGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path":        {req.Path},
		"path_hash":   {req.PathHash},
		"rx_pay":      {req.RxPay},
		"block_num":   {req.BlockNum},
		"num_blocks":  {req.NumBlocks},
		"read_marker": {req.ReadMarker},
		"auth_token":  {req.AuthToken},
		"content":     {req.AuthToken},
	}

	resp, err := DownloadHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.DownloadFileResponseCreator(resp), nil
}
