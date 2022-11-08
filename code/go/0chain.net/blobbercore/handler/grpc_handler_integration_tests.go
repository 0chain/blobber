//go:build integration_tests
// +build integration_tests

package handler

import (
	"context"
	"net/http"

	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/convert"
)

func (b *blobberGRPCService) ListEntities(ctx context.Context, req *blobbergrpc.ListEntitiesRequest) (*blobbergrpc.ListEntitiesResponse, error) {
	r, err := http.NewRequest("", "", http.NoBody)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, getGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path":       {req.Path},
		"path_hash":  {req.PathHash},
		"auth_token": {req.AuthToken},
	}

	resp, err, _ := ListHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.ListEntitesResponseCreator(resp), nil
}
