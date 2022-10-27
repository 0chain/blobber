package handler

import (
	"context"
	"net/http"
	"strings"

	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/convert"
)

type blobberGRPCService struct {
	blobbergrpc.UnimplementedBlobberServiceServer
}

func newGRPCBlobberService() *blobberGRPCService {
	return &blobberGRPCService{}
}

func (b *blobberGRPCService) GetAllocation(ctx context.Context, request *blobbergrpc.GetAllocationRequest) (*blobbergrpc.GetAllocationResponse, error) {
	r, err := http.NewRequest("GET", "", http.NoBody)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, getGRPCMetaDataFromCtx(ctx), "")
	r.Form = map[string][]string{"id": {request.Id}}

	resp, err := AllocationHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.GetAllocationResponseCreator(resp), nil
}

func (b *blobberGRPCService) GetFileMetaData(ctx context.Context, req *blobbergrpc.GetFileMetaDataRequest) (*blobbergrpc.GetFileMetaDataResponse, error) {
	r, err := http.NewRequest("POST", "", http.NoBody)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, getGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path_hash":  {req.PathHash},
		"path":       {req.Path},
		"auth_token": {req.AuthToken},
	}

	resp, err := FileMetaHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.GetFileMetaDataResponseCreator(resp), nil
}

func (b *blobberGRPCService) GetFileStats(ctx context.Context, req *blobbergrpc.GetFileStatsRequest) (*blobbergrpc.GetFileStatsResponse, error) {
	r, err := http.NewRequest("POST", "", http.NoBody)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, getGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path":      {req.Path},
		"path_hash": {req.PathHash},
	}

	resp, err := FileStatsHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.GetFileStatsResponseCreator(resp), nil
}

func (b *blobberGRPCService) GetReferencePath(ctx context.Context, req *blobbergrpc.GetReferencePathRequest) (*blobbergrpc.GetReferencePathResponse, error) {
	r, err := http.NewRequest("", "", http.NoBody)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, getGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path":  {req.Path},
		"paths": {req.Paths},
	}

	resp, err := ReferencePathHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.GetReferencePathResponseCreator(resp), nil
}

func (b *blobberGRPCService) GetObjectTree(ctx context.Context, req *blobbergrpc.GetObjectTreeRequest) (*blobbergrpc.GetObjectTreeResponse, error) {
	r, err := http.NewRequest("", "", http.NoBody)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, getGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path": {req.Path},
	}

	resp, _, err := ObjectTreeHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.GetObjectTreeResponseCreator(resp), nil
}

func (b *blobberGRPCService) CommitMetaTxn(ctx context.Context, req *blobbergrpc.CommitMetaTxnRequest) (*blobbergrpc.CommitMetaTxnResponse, error) {
	r, err := http.NewRequest("POST", "", http.NoBody)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, getGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path":       {req.Path},
		"path_hash":  {req.PathHash},
		"auth_token": {req.AuthToken},
		"txn_id":     {req.TxnId},
	}

	resp, err := CommitMetaTxnHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.GetCommitMetaTxnResponseCreator(resp), nil
}

func (b *blobberGRPCService) Collaborator(ctx context.Context, req *blobbergrpc.CollaboratorRequest) (*blobbergrpc.CollaboratorResponse, error) {
	r, err := http.NewRequest(strings.ToUpper(req.Method), "", http.NoBody)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, getGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path":      {req.Path},
		"path_hash": {req.PathHash},
		"collab_id": {req.CollabId},
	}

	var resp interface{}

	switch req.Method {
	case http.MethodPost:
		resp, err = AddCollaboratorHandler(ctx, r)
	case http.MethodGet:
		resp, err = GetCollaboratorHandler(ctx, r)
	case http.MethodDelete:
		resp, err = RemoveCollaboratorHandler(ctx, r)
	}

	if err != nil {
		return nil, err
	}

	return convert.CollaboratorResponseCreator(resp), nil
}
