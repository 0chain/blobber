package handler

import (
	"context"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/convert"
)

func (b *blobberGRPCService) Commit(ctx context.Context, req *blobbergrpc.CommitRequest) (*blobbergrpc.CommitResponse, error) {
	r, err := http.NewRequest("", "", nil)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, GetGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"write_marker":  {req.WriteMarker},
		"connection_id": {req.ConnectionId},
	}

	resp, err := CommitHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.CommitWriteResponseHandler(resp), nil
}
