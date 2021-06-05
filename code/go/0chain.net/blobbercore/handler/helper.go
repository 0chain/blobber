package handler

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
)

func RegisterGRPCServices(r *mux.Router, server *grpc.Server) {
	blobberService := newGRPCBlobberService(&storageHandler, &packageHandler{})
	grpcGatewayHandler := runtime.NewServeMux()

	blobbergrpc.RegisterBlobberServer(server, blobberService)
	_ = blobbergrpc.RegisterBlobberHandlerServer(context.Background(), grpcGatewayHandler, blobberService)
	r.PathPrefix("/").Handler(grpcGatewayHandler)
}

type StorageHandlerI interface {
	verifyAllocation(ctx context.Context, tx string, readonly bool) (alloc *allocation.Allocation, err error)
	verifyAuthTicket(ctx context.Context, authTokenString string, allocationObj *allocation.Allocation, refRequested *reference.Ref, clientID string) (bool, error)
}

// PackageHandler is an interface for all static functions that may need to be mocked
type PackageHandler interface {
	GetReference(ctx context.Context, allocationID string, newPath string) (*reference.Ref, error)
	GetReferenceLookup(ctx context.Context, allocationID string, path string) string
	GetReferenceFromLookupHash(ctx context.Context, allocationID string, path_hash string) (*reference.Ref, error)
	GetCommitMetaTxns(ctx context.Context, refID int64) ([]reference.CommitMetaTxn, error)
	AddCommitMetaTxn(ctx context.Context, refID int64, txnID string) error
	GetCollaborators(ctx context.Context, refID int64) ([]reference.Collaborator, error)
	IsACollaborator(ctx context.Context, refID int64, clientID string) bool
	GetFileStats(ctx context.Context, refID int64) (*stats.FileStats, error)
	GetWriteMarkerEntity(ctx context.Context, allocation_root string) (*writemarker.WriteMarkerEntity, error)
	GetRefWithChildren(ctx context.Context, allocationID string, path string) (*reference.Ref, error)
	GetObjectPath(ctx context.Context, allocationID string, blockNum int64) (*reference.ObjectPath, error)
	GetReferencePathFromPaths(ctx context.Context, allocationID string, paths []string) (*reference.Ref, error)
	GetAllocationChanges(ctx context.Context, connectionID string, allocationID string, clientID string) (*allocation.AllocationChangeCollector, error)
	SaveAllocationChanges(ctx context.Context, alloc *allocation.AllocationChangeCollector) error
	GetObjectTree(ctx context.Context, allocationID string, path string) (*reference.Ref, error)
	GetAllocationChanges(ctx context.Context, connectionID string, allocationID string, clientID string) (*allocation.AllocationChangeCollector, error)
	VerifyMarker(wm *writemarker.WriteMarkerEntity, ctx context.Context, sa *allocation.Allocation, co *allocation.AllocationChangeCollector) error
	ApplyChanges(connectionObj *allocation.AllocationChangeCollector, ctx context.Context, allocationRoot string) error
	GetReference(ctx context.Context, allocationID string, path string) (*reference.Ref, error)
	UpdateAllocationAndCommitChanges(ctx context.Context, writemarkerObj *writemarker.WriteMarkerEntity, connectionObj *allocation.AllocationChangeCollector, allocationObj *allocation.Allocation, allocationRoot string) error
}

type packageHandler struct{}

func (r *packageHandler) UpdateAllocationAndCommitChanges(ctx context.Context, writemarkerObj *writemarker.WriteMarkerEntity, connectionObj *allocation.AllocationChangeCollector, allocationObj *allocation.Allocation, allocationRoot string) error {
	return UpdateAllocationAndCommitChanges(ctx, writemarkerObj, connectionObj, allocationObj, allocationRoot)
}

func (r *packageHandler) GetReference(ctx context.Context, allocationID string, path string) (*reference.Ref, error) {
	return reference.GetReference(ctx, allocationID, path)
}

func (r *packageHandler) ApplyChanges(connectionObj *allocation.AllocationChangeCollector, ctx context.Context, allocationRoot string) error {
	return connectionObj.ApplyChanges(ctx, allocationRoot)
}

func (r *packageHandler) VerifyMarker(wm *writemarker.WriteMarkerEntity, ctx context.Context, sa *allocation.Allocation, co *allocation.AllocationChangeCollector) error {
	return wm.VerifyMarker(ctx, sa, co)
}

func (r *packageHandler) GetAllocationChanges(ctx context.Context, connectionID string, allocationID string, clientID string) (*allocation.AllocationChangeCollector, error) {
	return allocation.GetAllocationChanges(ctx, connectionID, allocationID, clientID)
}

func (r *packageHandler) GetObjectTree(ctx context.Context, allocationID string, path string) (*reference.Ref, error) {
	return reference.GetObjectTree(ctx, allocationID, path)
}

func (r *packageHandler) GetReferencePathFromPaths(ctx context.Context, allocationID string, paths []string) (*reference.Ref, error) {
	return reference.GetReferencePathFromPaths(ctx, allocationID, paths)
}

func (r *packageHandler) GetRefWithChildren(ctx context.Context, allocationID string, path string) (*reference.Ref, error) {
	return reference.GetRefWithChildren(ctx, allocationID, path)
}

func (r *packageHandler) GetObjectPath(ctx context.Context, allocationID string, blockNum int64) (*reference.ObjectPath, error) {
	return reference.GetObjectPath(ctx, allocationID, blockNum)
}

func (r *packageHandler) GetFileStats(ctx context.Context, refID int64) (*stats.FileStats, error) {
	return stats.GetFileStats(ctx, refID)
}

func (r *packageHandler) GetWriteMarkerEntity(ctx context.Context, allocation_root string) (*writemarker.WriteMarkerEntity, error) {
	return writemarker.GetWriteMarkerEntity(ctx, allocation_root)
}

func (r *packageHandler) GetReference(ctx context.Context, allocationID string, newPath string) (
	*reference.Ref, error) {
	return reference.GetReference(ctx, allocationID, newPath)
}

func (r *packageHandler) GetReferenceLookup(ctx context.Context, allocationID string, path string) string {
	return reference.GetReferenceLookup(allocationID, path)
}

func (r *packageHandler) GetReferenceFromLookupHash(ctx context.Context, allocationID string, path_hash string) (*reference.Ref, error) {
	return reference.GetReferenceFromLookupHash(ctx, allocationID, path_hash)
}

func (r *packageHandler) GetCommitMetaTxns(ctx context.Context, refID int64) ([]reference.CommitMetaTxn, error) {
	return reference.GetCommitMetaTxns(ctx, refID)
}

func (r *packageHandler) AddCommitMetaTxn(ctx context.Context, refID int64, txnID string) error {
	return reference.AddCommitMetaTxn(ctx, refID, txnID)
}

func (r *packageHandler) GetCollaborators(ctx context.Context, refID int64) ([]reference.Collaborator, error) {
	return reference.GetCollaborators(ctx, refID)
}

func (r *packageHandler) IsACollaborator(ctx context.Context, refID int64, clientID string) bool {
	return reference.IsACollaborator(ctx, refID, clientID)
}

func (r *packageHandler) GetAllocationChanges(ctx context.Context, connectionID string, allocationID string, clientID string) (*allocation.AllocationChangeCollector, error) {

	return allocation.GetAllocationChanges(ctx, connectionID, allocationID, clientID)
}

func (r packageHandler) SaveAllocationChanges(ctx context.Context, alloc *allocation.AllocationChangeCollector) error {
	return alloc.Save(ctx)
}
