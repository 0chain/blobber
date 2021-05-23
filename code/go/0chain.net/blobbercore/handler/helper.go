package handler

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/constants"
)

func setupGRPCHandlerContext(ctx context.Context, r *blobbergrpc.RequestContext) context.Context {
	ctx = context.WithValue(ctx, constants.CLIENT_CONTEXT_KEY,
		r.Client)
	ctx = context.WithValue(ctx, constants.CLIENT_KEY_CONTEXT_KEY,
		r.ClientKey)
	ctx = context.WithValue(ctx, constants.ALLOCATION_CONTEXT_KEY,
		r.Allocation)
	return ctx
}

func RegisterGRPCServices(r *mux.Router, server *grpc.Server) {
	packHandler := &packageHandler{}
	blobberService := newGRPCBlobberService(&storageHandler, packHandler)
	mux := runtime.NewServeMux()
	blobbergrpc.RegisterBlobberServer(server, blobberService)
	_ = blobbergrpc.RegisterBlobberHandlerServer(context.Background(), mux, blobberService)
	r.PathPrefix("/").Handler(mux)
}

type StorageHandlerI interface {
	verifyAllocation(ctx context.Context, tx string, readonly bool) (alloc *allocation.Allocation, err error)
	verifyAuthTicket(ctx context.Context, authTokenString string, allocationObj *allocation.Allocation, refRequested *reference.Ref, clientID string) (bool, error)
	readPreRedeem(ctx context.Context, alloc *allocation.Allocation,
		numBlocks, pendNumBlocks int64, payerID string) (err error)
}

// PackageHandler is an interface for all static functions that may need to be mocked
type PackageHandler interface {
	GetReferenceFromLookupHash(ctx context.Context, allocationID string, path_hash string) (*reference.Ref, error)
	GetReferenceLookup(ctx context.Context, allocationID string, path string) string
	GetCommitMetaTxns(ctx context.Context, refID int64) ([]reference.CommitMetaTxn, error)
	GetCollaborators(ctx context.Context, refID int64) ([]reference.Collaborator, error)
	IsACollaborator(ctx context.Context, refID int64, clientID string) bool
	GetFileStats(ctx context.Context, refID int64) (*stats.FileStats, error)
	GetWriteMarkerEntity(ctx context.Context, allocation_root string) (*writemarker.WriteMarkerEntity, error)
	GetRefWithChildren(ctx context.Context, allocationID string, path string) (*reference.Ref, error)
	GetObjectPathGRPC(ctx context.Context, allocationID string, blockNum int64) (*blobbergrpc.ObjectPath, error)
	GetReferencePathFromPaths(ctx context.Context, allocationID string, paths []string) (*reference.Ref, error)
	GetObjectTree(ctx context.Context, allocationID string, path string) (*reference.Ref, error)
	GetAllocationChanges(ctx context.Context, connectionID string,
		allocationID string, clientID string) (allocation.IAllocationChangeCollector, error)
	GetReference(ctx context.Context, allocationID string, newPath string) (
		*reference.Ref, error)
	// write readmeker interface separately and add these two methods
	GetLatestReadMarkerEntity(ctx context.Context, clientID string) (*readmarker.ReadMarkerEntity, error)
	SaveLatestReadMarker(ctx context.Context, rm *readmarker.ReadMarker, isCreate bool) error
	// write FileStat related methods in a different interface
	GetFileStore() filestore.FileStore
	FileBlockDownloaded(ctx context.Context, refID int64)
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

func (r *packageHandler) GetReferenceLookup(ctx context.Context, allocationID string, path string) string {
	return reference.GetReferenceLookup(allocationID, path)
}

func (r *packageHandler) GetReference(ctx context.Context, allocationID string, newPath string) (
	*reference.Ref, error) {
	return reference.GetReference(ctx, allocationID, newPath)
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

func (r *packageHandler) GetAllocationChanges(ctx context.Context, connectionID string,
	allocationID string, clientID string) (allocation.IAllocationChangeCollector, error) {

	return allocation.GetAllocationChanges(ctx, connectionID, allocationID, clientID)
}

func (r *packageHandler) GetLatestReadMarkerEntity(ctx context.Context, clientID string) (
	*readmarker.ReadMarkerEntity, error) {

	return readmarker.GetLatestReadMarkerEntity(ctx, clientID)
}

func (r *packageHandler) SaveLatestReadMarker(ctx context.Context, rm *readmarker.ReadMarker, isCreate bool) error {
	return readmarker.SaveLatestReadMarker(ctx, rm, isCreate)
}

func (r *packageHandler) GetFileStore() filestore.FileStore {
	return filestore.GetFileStore()
}

func (r *packageHandler) FileBlockDownloaded(ctx context.Context, refID int64) {
	stats.FileBlockDownloaded(ctx, refID)
}
