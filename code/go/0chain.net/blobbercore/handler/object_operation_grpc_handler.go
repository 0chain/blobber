package handler

import (
	"context"
	"net/http"

	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/convert"
)

func (b *blobberGRPCService) CopyObject(ctx context.Context, req *blobbergrpc.CopyObjectRequest) (*blobbergrpc.CopyObjectResponse, error) {
	r, err := http.NewRequest("POST", "", http.NoBody)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, getGRPCMetaDataFromCtx(ctx), req.Allocation)
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
	r, err := http.NewRequest("POST", "", http.NoBody)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, getGRPCMetaDataFromCtx(ctx), req.Allocation)
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
