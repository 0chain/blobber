package handler

import (
	"context"
	"strconv"

	"0chain.net/blobbercore/blobbergrpc/converter"

	"0chain.net/blobbercore/stats"
	"0chain.net/blobbercore/writemarker"

	"0chain.net/blobbercore/reference"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"

	"0chain.net/core/common"

	"go.uber.org/zap"

	"github.com/gorilla/mux"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"

	"google.golang.org/grpc"

	"0chain.net/blobbercore/blobbergrpc"
)

type blobberGRPCService struct {
	storageHandler StorageHandler
	blobbergrpc.UnimplementedBlobberServer
}

func RegisterGRPCServices(r *mux.Router, server *grpc.Server) {
	blobberService := newGRPCBlobberService()
	mux := runtime.NewServeMux()
	blobbergrpc.RegisterBlobberServer(server, blobberService)
	blobbergrpc.RegisterBlobberHandlerServer(context.Background(), mux, blobberService)
	r.PathPrefix("/").Handler(mux)
}

func newGRPCBlobberService() *blobberGRPCService {
	return &blobberGRPCService{}
}

func (b *blobberGRPCService) GetAllocation(ctx context.Context, request *blobbergrpc.GetAllocationRequest) (*blobbergrpc.GetAllocationResponse, error) {
	ctx = setupGRPCHandlerContext(ctx, request.Context)

	allocation, err := b.storageHandler.verifyAllocation(ctx, request.Id, false)
	if err != nil {
		return nil, err
	}

	return &blobbergrpc.GetAllocationResponse{Allocation: converter.AllocationToGRPCAllocation(allocation)}, nil
}

func (b *blobberGRPCService) GetFileMetaData(ctx context.Context, req *blobbergrpc.GetFileMetaDataRequest) (*blobbergrpc.GetFileMetaDataResponse, error) {
	logger := ctxzap.Extract(ctx)
	allocationObj, err := b.storageHandler.verifyAllocation(ctx, req.Allocation, true)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}
	allocationID := allocationObj.ID

	clientID := req.Context.Client
	if len(clientID) == 0 {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	path_hash := req.PathHash
	path := req.Path
	if len(path_hash) == 0 {
		if len(path) == 0 {
			return nil, common.NewError("invalid_parameters", "Invalid path")
		}
		path_hash = reference.GetReferenceLookup(allocationID, path)
	}

	fileref, err := reference.GetReferenceFromLookupHash(ctx, allocationID, path_hash)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	if fileref.Type != reference.FILE {
		return nil, common.NewError("invalid_parameters", "Path is not a file.")
	}

	commitMetaTxns, err := reference.GetCommitMetaTxns(ctx, fileref.ID)
	if err != nil {
		logger.Error("Failed to get commitMetaTxns from refID", zap.Error(err), zap.Any("ref_id", fileref.ID))
	}
	fileref.CommitMetaTxns = commitMetaTxns

	collaborators, err := reference.GetCollaborators(ctx, fileref.ID)
	if err != nil {
		logger.Error("Failed to get collaborators from refID", zap.Error(err), zap.Any("ref_id", fileref.ID))
	}

	authTokenString := req.AuthToken

	if (allocationObj.OwnerID != clientID &&
		allocationObj.PayerID != clientID &&
		!reference.IsACollaborator(ctx, fileref.ID, clientID)) || len(authTokenString) > 0 {
		authTicketVerified, err := b.storageHandler.verifyAuthTicket(ctx, req.AuthToken, allocationObj, fileref, clientID)
		if err != nil {
			return nil, err
		}
		if !authTicketVerified {
			return nil, common.NewError("auth_ticket_verification_failed", "Could not verify the auth ticket.")
		}
		fileref.Path = ""
	}

	var collaboratorsGRPC []*blobbergrpc.Collaborator
	for _, c := range collaborators {
		collaboratorsGRPC = append(collaboratorsGRPC, &blobbergrpc.Collaborator{
			RefId:     c.RefID,
			ClientId:  c.ClientID,
			CreatedAt: c.CreatedAt.UnixNano(),
		})
	}

	return &blobbergrpc.GetFileMetaDataResponse{
		MetaData:      converter.FileRefToFileRefGRPC(fileref),
		Collaborators: collaboratorsGRPC,
	}, nil
}

func (b *blobberGRPCService) GetFileStats(ctx context.Context, req *blobbergrpc.GetFileStatsRequest) (*blobbergrpc.GetFileStatsResponse, error) {
	allocationTx := req.Context.Allocation
	allocationObj, err := b.storageHandler.verifyAllocation(ctx, allocationTx, true)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}
	allocationID := allocationObj.ID

	clientID := req.Context.Client
	if len(clientID) == 0 || allocationObj.OwnerID != clientID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	path_hash := req.PathHash
	path := req.Path
	if len(path_hash) == 0 {
		if len(path) == 0 {
			return nil, common.NewError("invalid_parameters", "Invalid path")
		}
		path_hash = reference.GetReferenceLookup(allocationID, path)
	}

	fileref, err := reference.GetReferenceFromLookupHash(ctx, allocationID, path_hash)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	if fileref.Type != reference.FILE {
		return nil, common.NewError("invalid_parameters", "Path is not a file.")
	}

	stats, _ := stats.GetFileStats(ctx, fileref.ID)
	wm, _ := writemarker.GetWriteMarkerEntity(ctx, fileref.WriteMarker)
	if wm != nil && stats != nil {
		stats.WriteMarkerRedeemTxn = wm.CloseTxnID
	}

	return &blobbergrpc.GetFileStatsResponse{
		MetaData: converter.FileRefToFileRefGRPC(fileref),
		Stats:    converter.FileStatsToFileStatsGRPC(stats),
	}, nil
}

func (b *blobberGRPCService) ListEntities(ctx context.Context, req *blobbergrpc.ListEntitiesRequest) (*blobbergrpc.ListEntitiesResponse, error) {
	logger := ctxzap.Extract(ctx)

	clientID := req.Context.Client
	allocationTx := req.Context.Allocation
	allocationObj, err := b.storageHandler.verifyAllocation(ctx, allocationTx, true)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}
	allocationID := allocationObj.ID

	if len(clientID) == 0 {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	path_hash := req.PathHash
	path := req.Path
	if len(path_hash) == 0 {
		if len(path) == 0 {
			return nil, common.NewError("invalid_parameters", "Invalid path")
		}
		path_hash = reference.GetReferenceLookup(allocationID, path)
	}

	logger.Info("Path Hash for list dir :" + path_hash)

	fileref, err := reference.GetReferenceFromLookupHash(ctx, allocationID, path_hash)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid path. "+err.Error())
	}
	authTokenString := req.AuthToken
	if clientID != allocationObj.OwnerID || len(authTokenString) > 0 {
		authTicketVerified, err := b.storageHandler.verifyAuthTicket(ctx, authTokenString, allocationObj, fileref, clientID)
		if err != nil {
			return nil, err
		}
		if !authTicketVerified {
			return nil, common.NewError("auth_ticket_verification_failed", "Could not verify the auth ticket.")
		}
	}

	dirref, err := reference.GetRefWithChildren(ctx, allocationID, fileref.Path)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid path. "+err.Error())
	}

	if clientID != allocationObj.OwnerID {
		dirref.Path = ""
	}

	var entities []*blobbergrpc.FileRef
	for _, entity := range dirref.Children {
		if clientID != allocationObj.OwnerID {
			entity.Path = ""
		}
		entities = append(entities, converter.FileRefToFileRefGRPC(entity))
	}
	refGRPC := converter.FileRefToFileRefGRPC(dirref)
	refGRPC.DirMetaData.Children = entities

	return &blobbergrpc.ListEntitiesResponse{
		AllocationRoot: allocationObj.AllocationRoot,
		MetaData:       refGRPC,
	}, nil
}

func (b *blobberGRPCService) GetObjectPath(ctx context.Context, req *blobbergrpc.GetObjectPathRequest) (*blobbergrpc.GetObjectPathResponse, error) {
	allocationTx := req.Context.Allocation
	allocationObj, err := b.storageHandler.verifyAllocation(ctx, allocationTx, false)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}
	allocationID := allocationObj.ID

	clientID := req.Context.Client
	if len(clientID) == 0 || allocationObj.OwnerID != clientID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}
	path := req.Path
	if len(path) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid path")
	}

	blockNumStr := req.BlockNum
	if len(blockNumStr) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid path")
	}

	blockNum, err := strconv.ParseInt(blockNumStr, 10, 64)
	if err != nil || blockNum < 0 {
		return nil, common.NewError("invalid_parameters", "Invalid block number")
	}

	objectPath, err := reference.GetObjectPathGRPC(ctx, allocationID, blockNum)
	if err != nil {
		return nil, err
	}

	var latestWM *writemarker.WriteMarkerEntity
	if len(allocationObj.AllocationRoot) == 0 {
		latestWM = nil
	} else {
		latestWM, err = writemarker.GetWriteMarkerEntity(ctx, allocationObj.AllocationRoot)
		if err != nil {
			return nil, common.NewError("latest_write_marker_read_error", "Error reading the latest write marker for allocation."+err.Error())
		}
	}
	var latestWriteMarketGRPC *blobbergrpc.WriteMarker
	if latestWM != nil {
		latestWriteMarketGRPC = converter.WriteMarkerToWriteMarkerGRPC(latestWM.WM)
	}
	return &blobbergrpc.GetObjectPathResponse{
		ObjectPath:        objectPath,
		LatestWriteMarker: latestWriteMarketGRPC,
	}, nil
}
