package handler

import (
	"context"
	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"github.com/pkg/errors"
	"net/http"
	"strings"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/convert"
)

type blobberGRPCService struct {
	blobbergrpc.UnimplementedBlobberServiceServer
}

func newGRPCBlobberService() *blobberGRPCService {
	return &blobberGRPCService{}
}

func (b *blobberGRPCService) GetAllocation(ctx context.Context, request *blobbergrpc.GetAllocationRequest) (*blobbergrpc.GetAllocationResponse, error) {
	ctx = setupGrpcHandlerContext(ctx, getGRPCMetaDataFromCtx(ctx))
	response, err := storageHandler.GetAllocationDetails(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get allocation details for request: " + request.String())
	}

	return convert.GetAllocationResponseCreator(response), nil
}

func (b *blobberGRPCService) GetFileMetaData(ctx context.Context, request *blobbergrpc.GetFileMetaDataRequest) (*blobbergrpc.GetFileMetaDataResponse, error) {
	ctx = setupGrpcHandlerContext(ctx, getGRPCMetaDataFromCtx(ctx))

	response, err := storageHandler.GetFileMeta(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get FileMetadata for request: " + request.String())
	}

	return convert.GetFileMetaDataResponseCreator(response), nil
}

func (b *blobberGRPCService) GetFileStats(ctx context.Context, req *blobbergrpc.GetFileStatsRequest) (*blobbergrpc.GetFileStatsResponse, error) {
	r, err := http.NewRequest("POST", "", nil)
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

func (b *blobberGRPCService) ListEntities(ctx context.Context, request *blobbergrpc.ListEntitiesRequest) (*blobbergrpc.ListEntitiesResponse, error) {
	ctx = setupGrpcHandlerContext(ctx, getGRPCMetaDataFromCtx(ctx))

	response, err := storageHandler.ListEntities(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get list entities")
	}

	return convert.ListEntitesResponseCreator(response), nil
}

func (b *blobberGRPCService) GetObjectPath(ctx context.Context, request *blobbergrpc.GetObjectPathRequest) (*blobbergrpc.GetObjectPathResponse, error) {
	ctx = setupGrpcHandlerContext(ctx, getGRPCMetaDataFromCtx(ctx))


	response, err := storageHandler.GetObjectPath(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to GetObjectPath")
	}

	return convert.GetObjectPathResponseCreator(response), nil
}

func (b *blobberGRPCService) GetReferencePath(ctx context.Context, request *blobbergrpc.GetReferencePathRequest) (*blobbergrpc.GetReferencePathResponse, error) {

	ctx = setupGrpcHandlerContext(ctx, getGRPCMetaDataFromCtx(ctx))
	ctx, canceler := context.WithTimeout(ctx, time.Second*10)
	defer canceler()

	response, err := storageHandler.GetReferencePath(ctx, request)

	if err != nil {
		return nil, errors.Wrap(err, "failed to GetReferencePath")
	}

	return convert.GetReferencePathResponseCreator(response), nil
}

func (b *blobberGRPCService) GetObjectTree(ctx context.Context, request *blobbergrpc.GetObjectTreeRequest) (*blobbergrpc.GetObjectTreeResponse, error) {

	ctx = setupGrpcHandlerContext(ctx, getGRPCMetaDataFromCtx(ctx))

	response, err := storageHandler.GetObjectTree(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to GetObjectTree")
	}

	return convert.GetObjectTreeResponseCreator(response), nil
}

func (b *blobberGRPCService) CalculateHash(ctx context.Context, req *blobbergrpc.CalculateHashRequest) (*blobbergrpc.CalculateHashResponse, error) {
	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, getGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path":  {req.Path},
		"paths": {req.Paths},
	}

	resp, err := CalculateHashHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.GetCalculateHashResponseCreator(resp), nil
}

func (b *blobberGRPCService) CommitMetaTxn(ctx context.Context, req *blobbergrpc.CommitMetaTxnRequest) (*blobbergrpc.CommitMetaTxnResponse, error) {
	r, err := http.NewRequest("POST", "", nil)
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
	r, err := http.NewRequest(strings.ToUpper(req.Method), "", nil)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, getGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path":      {req.Path},
		"path_hash": {req.PathHash},
		"collab_id": {req.CollabId},
	}

	resp, err := CollaboratorHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.CollaboratorResponseCreator(resp), nil
}
