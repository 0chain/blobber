package handler

import (
	"context"
	"encoding/json"
	"strconv"

	"0chain.net/blobbercore/allocation"

	"0chain.net/blobbercore/stats"
	"0chain.net/blobbercore/writemarker"

	"0chain.net/blobbercore/reference"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"

	"0chain.net/core/common"

	"go.uber.org/zap"

	"github.com/gorilla/mux"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	"google.golang.org/grpc"

	"0chain.net/blobbercore/blobbergrpc"
)

type StorageHandlerI interface {
	verifyAllocation(ctx context.Context, tx string, readonly bool) (alloc *allocation.Allocation, err error)
	verifyAuthTicket(ctx context.Context, authTokenString string, allocationObj *allocation.Allocation, refRequested *reference.Ref, clientID string) (bool, error)
}

// PackageHandler is an interface for all static functions that may need to be mocked
type PackageHandler interface {
	GetReferenceFromLookupHash(ctx context.Context, allocationID string, path_hash string) (*reference.Ref, error)
	GetCommitMetaTxns(ctx context.Context, refID int64) ([]reference.CommitMetaTxn, error)
	GetCollaborators(ctx context.Context, refID int64) ([]reference.Collaborator, error)
	IsACollaborator(ctx context.Context, refID int64, clientID string) bool
	GetFileStats(ctx context.Context, refID int64) (*stats.FileStats, error)
	GetWriteMarkerEntity(ctx context.Context, allocation_root string) (*writemarker.WriteMarkerEntity, error)
	GetRefWithChildren(ctx context.Context, allocationID string, path string) (*reference.Ref, error)
	GetObjectPathGRPC(ctx context.Context, allocationID string, blockNum int64) (*blobbergrpc.ObjectPath, error)
	GetReferencePathFromPaths(ctx context.Context, allocationID string, paths []string) (*reference.Ref, error)
	GetObjectTree(ctx context.Context, allocationID string, path string) (*reference.Ref, error)
}

type packageHandler struct{}

func (r *packageHandler) GetObjectTree(ctx context.Context, allocationID string, path string) (*reference.Ref, error) {
	return reference.GetObjectTree(ctx, allocationID, path)
}

func (r *packageHandler) GetReferencePathFromPaths(ctx context.Context, allocationID string, paths []string) (*reference.Ref, error) {
	return reference.GetReferencePathFromPaths(ctx, allocationID, paths)
}

func (r *packageHandler) GetRefWithChildren(ctx context.Context, allocationID string, path string) (*reference.Ref, error) {
	return reference.GetRefWithChildren(ctx, allocationID, path)
}

func (r *packageHandler) GetObjectPathGRPC(ctx context.Context, allocationID string, blockNum int64) (*blobbergrpc.ObjectPath, error) {
	return reference.GetObjectPathGRPC(ctx, allocationID, blockNum)
}

func (r *packageHandler) GetFileStats(ctx context.Context, refID int64) (*stats.FileStats, error) {
	return stats.GetFileStats(ctx, refID)
}

func (r *packageHandler) GetWriteMarkerEntity(ctx context.Context, allocation_root string) (*writemarker.WriteMarkerEntity, error) {
	return writemarker.GetWriteMarkerEntity(ctx, allocation_root)
}

func (r *packageHandler) GetReferenceFromLookupHash(ctx context.Context, allocationID string, path_hash string) (*reference.Ref, error) {
	return reference.GetReferenceFromLookupHash(ctx, allocationID, path_hash)
}

func (r *packageHandler) GetCommitMetaTxns(ctx context.Context, refID int64) ([]reference.CommitMetaTxn, error) {
	return reference.GetCommitMetaTxns(ctx, refID)
}

func (r *packageHandler) GetCollaborators(ctx context.Context, refID int64) ([]reference.Collaborator, error) {
	return reference.GetCollaborators(ctx, refID)
}

func (r *packageHandler) IsACollaborator(ctx context.Context, refID int64, clientID string) bool {
	return reference.IsACollaborator(ctx, refID, clientID)
}

type blobberGRPCService struct {
	storageHandler StorageHandlerI
	packageHandler PackageHandler
	blobbergrpc.UnimplementedBlobberServer
}

func RegisterGRPCServices(r *mux.Router, server *grpc.Server) {
	packHandler := &packageHandler{}
	blobberService := newGRPCBlobberService(&storageHandler, packHandler)
	mux := runtime.NewServeMux()
	blobbergrpc.RegisterBlobberServer(server, blobberService)
	blobbergrpc.RegisterBlobberHandlerServer(context.Background(), mux, blobberService)
	r.PathPrefix("/").Handler(mux)
}

func newGRPCBlobberService(sh StorageHandlerI, r PackageHandler) *blobberGRPCService {
	return &blobberGRPCService{
		storageHandler: sh,
		packageHandler: r,
	}
}

func (b *blobberGRPCService) GetAllocation(ctx context.Context, request *blobbergrpc.GetAllocationRequest) (*blobbergrpc.GetAllocationResponse, error) {
	ctx = setupGRPCHandlerContext(ctx, request.Context)

	allocation, err := b.storageHandler.verifyAllocation(ctx, request.Id, false)
	if err != nil {
		return nil, err
	}

	return &blobbergrpc.GetAllocationResponse{Allocation: AllocationToGRPCAllocation(allocation)}, nil
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

	fileref, err := b.packageHandler.GetReferenceFromLookupHash(ctx, allocationID, path_hash)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	if fileref.Type != reference.FILE {
		return nil, common.NewError("invalid_parameters", "Path is not a file.")
	}

	commitMetaTxns, err := b.packageHandler.GetCommitMetaTxns(ctx, fileref.ID)
	if err != nil {
		logger.Error("Failed to get commitMetaTxns from refID", zap.Error(err), zap.Any("ref_id", fileref.ID))
	}
	fileref.CommitMetaTxns = commitMetaTxns

	collaborators, err := b.packageHandler.GetCollaborators(ctx, fileref.ID)
	if err != nil {
		logger.Error("Failed to get collaborators from refID", zap.Error(err), zap.Any("ref_id", fileref.ID))
	}

	authTokenString := req.AuthToken

	if (allocationObj.OwnerID != clientID &&
		allocationObj.PayerID != clientID &&
		!b.packageHandler.IsACollaborator(ctx, fileref.ID, clientID)) || len(authTokenString) > 0 {
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
		MetaData:      reference.FileRefToFileRefGRPC(fileref),
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

	fileref, err := b.packageHandler.GetReferenceFromLookupHash(ctx, allocationID, path_hash)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	if fileref.Type != reference.FILE {
		return nil, common.NewError("invalid_parameters", "Path is not a file.")
	}

	stats, _ := b.packageHandler.GetFileStats(ctx, fileref.ID)
	wm, _ := b.packageHandler.GetWriteMarkerEntity(ctx, fileref.WriteMarker)
	if wm != nil && stats != nil {
		stats.WriteMarkerRedeemTxn = wm.CloseTxnID
	}

	return &blobbergrpc.GetFileStatsResponse{
		MetaData: reference.FileRefToFileRefGRPC(fileref),
		Stats:    FileStatsToFileStatsGRPC(stats),
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

	fileref, err := b.packageHandler.GetReferenceFromLookupHash(ctx, allocationID, path_hash)
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

	dirref, err := b.packageHandler.GetRefWithChildren(ctx, allocationID, fileref.Path)
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
		entities = append(entities, reference.FileRefToFileRefGRPC(entity))
	}
	refGRPC := reference.FileRefToFileRefGRPC(dirref)
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

	objectPath, err := b.packageHandler.GetObjectPathGRPC(ctx, allocationID, blockNum)
	if err != nil {
		return nil, err
	}

	var latestWM *writemarker.WriteMarkerEntity
	if len(allocationObj.AllocationRoot) == 0 {
		latestWM = nil
	} else {
		latestWM, err = b.packageHandler.GetWriteMarkerEntity(ctx, allocationObj.AllocationRoot)
		if err != nil {
			return nil, common.NewError("latest_write_marker_read_error", "Error reading the latest write marker for allocation."+err.Error())
		}
	}
	var latestWriteMarketGRPC *blobbergrpc.WriteMarker
	if latestWM != nil {
		latestWriteMarketGRPC = WriteMarkerToWriteMarkerGRPC(latestWM.WM)
	}
	return &blobbergrpc.GetObjectPathResponse{
		ObjectPath:        objectPath,
		LatestWriteMarker: latestWriteMarketGRPC,
	}, nil
}

func (b *blobberGRPCService) GetReferencePath(ctx context.Context, req *blobbergrpc.GetReferencePathRequest) (*blobbergrpc.GetReferencePathResponse, error) {

	allocationTx := req.Context.Allocation
	allocationObj, err := b.storageHandler.verifyAllocation(ctx, allocationTx, false)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}
	allocationID := allocationObj.ID

	clientID := req.Context.Client
	if len(clientID) == 0 {
		return nil, common.NewError("invalid_operation", "Please pass clientID in the header")
	}

	var paths []string
	pathsString := req.Paths
	if len(pathsString) == 0 {
		path := req.Path
		if len(path) == 0 {
			return nil, common.NewError("invalid_parameters", "Invalid path")
		}
		paths = append(paths, path)
	} else {
		err = json.Unmarshal([]byte(pathsString), &paths)
		if err != nil {
			return nil, common.NewError("invalid_parameters", "Invalid path array json")
		}
	}

	rootRef, err := b.packageHandler.GetReferencePathFromPaths(ctx, allocationID, paths)
	if err != nil {
		return nil, err
	}

	refPath := &blobbergrpc.ReferencePath{MetaData: reference.FileRefToFileRefGRPC(rootRef)}
	refsToProcess := make([]*blobbergrpc.ReferencePath, 0)
	refsToProcess = append(refsToProcess, refPath)
	for len(refsToProcess) > 0 {
		refToProcess := refsToProcess[0]
		if len(refToProcess.MetaData.DirMetaData.Children) > 0 {
			refToProcess.List = make([]*blobbergrpc.ReferencePath, len(refToProcess.MetaData.DirMetaData.Children))
		}
		for idx, child := range refToProcess.MetaData.DirMetaData.Children {
			childRefPath := &blobbergrpc.ReferencePath{MetaData: child}
			refToProcess.List[idx] = childRefPath
			refsToProcess = append(refsToProcess, childRefPath)
		}
		refsToProcess = refsToProcess[1:]
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
	var refPathResult blobbergrpc.GetReferencePathResponse
	refPathResult.ReferencePath = refPath
	if latestWM != nil {
		refPathResult.LatestWM = WriteMarkerToWriteMarkerGRPC(latestWM.WM)
	}

	return &refPathResult, nil
}

func (b *blobberGRPCService) GetObjectTree(ctx context.Context, req *blobbergrpc.GetObjectTreeRequest) (*blobbergrpc.GetObjectTreeResponse, error) {
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

	rootRef, err := b.packageHandler.GetObjectTree(ctx, allocationID, path)
	if err != nil {
		return nil, err
	}

	refPath := &blobbergrpc.ReferencePath{MetaData: reference.FileRefToFileRefGRPC(rootRef)}
	refsToProcess := make([]*blobbergrpc.ReferencePath, 0)
	refsToProcess = append(refsToProcess, refPath)
	for len(refsToProcess) > 0 {
		refToProcess := refsToProcess[0]
		if len(refToProcess.MetaData.DirMetaData.Children) > 0 {
			refToProcess.List = make([]*blobbergrpc.ReferencePath, len(refToProcess.MetaData.DirMetaData.Children))
		}
		for idx, child := range refToProcess.MetaData.DirMetaData.Children {
			childRefPath := &blobbergrpc.ReferencePath{MetaData: child}
			refToProcess.List[idx] = childRefPath
			refsToProcess = append(refsToProcess, childRefPath)
		}
		refsToProcess = refsToProcess[1:]
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
	var refPathResult blobbergrpc.GetObjectTreeResponse
	refPathResult.ReferencePath = refPath
	if latestWM != nil {
		refPathResult.LatestWM = WriteMarkerToWriteMarkerGRPC(latestWM.WM)
	}
	return &refPathResult, nil
}
