package handler

import (
	"context"
	"time"

	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/pkg/errors"
	"go.uber.org/zap"

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
		return nil, errors.Wrap(err, "unable to get allocation details for request: "+request.String())
	}

	return convert.GetAllocationResponseCreator(response), nil
}

func (b *blobberGRPCService) GetFileMetaData(ctx context.Context, request *blobbergrpc.GetFileMetaDataRequest) (*blobbergrpc.GetFileMetaDataResponse, error) {
	ctx = setupGrpcHandlerContext(ctx, getGRPCMetaDataFromCtx(ctx))

	response, err := storageHandler.GetFileMeta(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get FileMetadata for request: "+request.String())
	}

	return convert.GetFileMetaDataResponseCreator(response), nil
}

func (b *blobberGRPCService) GetFileStats(ctx context.Context, request *blobbergrpc.GetFileStatsRequest) (*blobbergrpc.GetFileStatsResponse, error) {
	ctx = setupGrpcHandlerContext(ctx, getGRPCMetaDataFromCtx(ctx))

	response, err := storageHandler.GetFileStats(ctx, request)
	if err != nil {
		logging.Logger.Info("failed with error", zap.Error(err))
		return nil, errors.Wrap(err, "failed to get FileStats for request: "+request.String())
	}

	result, err := convert.GetFileStatsResponseCreator(response)
	if err != nil {
		logging.Logger.Info("failed with error", zap.Error(err))
		return nil, errors.Wrap(err, "failed to convert FileStats for request: "+request.String())
	}

	return result, nil
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

	ctx = GetMetaDataStore().CreateTransaction(ctx)
	defer func() {
		GetMetaDataStore().GetTransaction(ctx).Rollback()
	}()
	ctx = setupGrpcHandlerContext(ctx, getGRPCMetaDataFromCtx(ctx))
	ctx, canceler := context.WithTimeout(ctx, time.Second*10)
	defer canceler()

	logging.Logger.Info("handler GetReferencePath req info",
		zap.String("allocation", request.Allocation),
		zap.String("request", request.String()))

	logging.Logger.Info("call storageHandler.GetReferencePath",
		zap.Any("request", request.String()))
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

func (b *blobberGRPCService) CalculateHash(ctx context.Context, request *blobbergrpc.CalculateHashRequest) (*blobbergrpc.CalculateHashResponse, error) {

	ctx = setupGrpcHandlerContext(ctx, getGRPCMetaDataFromCtx(ctx))

	response, err := storageHandler.CalculateHash(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to CalculateHash")
	}

	return response, nil
}

func (b *blobberGRPCService) CommitMetaTxn(ctx context.Context, request *blobbergrpc.CommitMetaTxnRequest) (*blobbergrpc.CommitMetaTxnResponse, error) {

	ctx = setupGrpcHandlerContext(ctx, getGRPCMetaDataFromCtx(ctx))

	response, err := storageHandler.AddCommitMetaTxn(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to CommitMetaTxn")
	}

	return response, nil
}

func (b *blobberGRPCService) Collaborator(ctx context.Context, request *blobbergrpc.CollaboratorRequest) (*blobbergrpc.CollaboratorResponse, error) {
	ctx = setupGrpcHandlerContext(ctx, getGRPCMetaDataFromCtx(ctx))

	response, err := storageHandler.AddCollaborator(ctx, request)

	if err != nil {
		return nil, errors.Wrap(err, "failed to ModifyCollaborators")
	}
	return response, nil
}

func (b *blobberGRPCService) MarketplaceShareInfo(ctx context.Context, request *blobbergrpc.MarketplaceShareInfoRequest) (*blobbergrpc.MarketplaceShareInfoResponse, error) {
	ctx = setupGrpcHandlerContext(ctx, getGRPCMetaDataFromCtx(ctx))

	response, err := storageHandler.MarketplaceShareInfo(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, "failed revoke/insert market place share")
	}

	return response, nil
}
