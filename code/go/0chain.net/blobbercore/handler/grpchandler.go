package handler

import (
	"context"

	"0chain.net/blobbercore/reference"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"

	"0chain.net/core/common"

	"go.uber.org/zap"

	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"

	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"

	"github.com/gorilla/mux"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"

	"google.golang.org/grpc"

	"0chain.net/blobbercore/blobbergrpc"
	"0chain.net/core/logging"
)

type blobberGRPCService struct {
	storageHandler StorageHandler
	blobbergrpc.UnimplementedBlobberServer
}

func unaryDatabaseTransactionInjector() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		logger := ctxzap.Extract(ctx)

		ctx = GetMetaDataStore().CreateTransaction(ctx)
		resp, err := handler(ctx, req)
		if err != nil {
			var rollErr = GetMetaDataStore().GetTransaction(ctx).Rollback().Error
			if rollErr != nil {
				logger.Error("couldn't rollback", zap.Error(err))
			}
			return nil, err
		}

		err = GetMetaDataStore().GetTransaction(ctx).Commit().Error
		if err != nil {
			return nil, common.NewErrorf("commit_error",
				"error committing to meta store: %v", err)
		}

		return resp, err
	}
}

func NewServerWithMiddlewares() *grpc.Server {
	return grpc.NewServer(
		grpc.ChainStreamInterceptor(
			grpc_zap.StreamServerInterceptor(logging.Logger),
			grpc_recovery.StreamServerInterceptor(),
		),
		grpc.ChainUnaryInterceptor(
			grpc_zap.UnaryServerInterceptor(logging.Logger),
			grpc_recovery.UnaryServerInterceptor(),
			unaryDatabaseTransactionInjector(),
		),
	)
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

	return &blobbergrpc.GetAllocationResponse{Allocation: convertAllocationToGRPCAllocation(allocation)}, nil
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

	var commitMetaTxnsGRPC []*blobbergrpc.CommitMetaTxn
	for _, c := range commitMetaTxns {
		commitMetaTxnsGRPC = append(commitMetaTxnsGRPC, &blobbergrpc.CommitMetaTxn{
			RefId:     c.RefID,
			TxnId:     c.TxnID,
			CreatedAt: c.CreatedAt.UnixNano(),
		})
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
		Type:                fileref.Type,
		LookupHash:          fileref.LookupHash,
		Name:                fileref.Name,
		Path:                fileref.Path,
		Hash:                fileref.Hash,
		NumBlocks:           fileref.NumBlocks,
		PathHash:            fileref.PathHash,
		CustomMeta:          fileref.CustomMeta,
		ContentHash:         fileref.ContentHash,
		Size:                fileref.Size,
		MerkleRoot:          fileref.MerkleRoot,
		ActualFileSize:      fileref.ActualFileSize,
		ActualFileHash:      fileref.ActualFileHash,
		MimeType:            fileref.MimeType,
		ThumbnailSize:       fileref.ThumbnailSize,
		ThumbnailHash:       fileref.ThumbnailHash,
		ActualThumbnailSize: fileref.ActualThumbnailSize,
		ActualThumbnailHash: fileref.ActualThumbnailHash,
		EncryptedKey:        fileref.EncryptedKey,
		Attributes:          fileref.Attributes,
		OnCloud:             fileref.OnCloud,
		CommitMetaTxns:      commitMetaTxnsGRPC,
		CreatedAt:           fileref.CreatedAt.UnixNano(),
		UpdatedAt:           fileref.UpdatedAt.UnixNano(),
		Collaborators:       collaboratorsGRPC,
	}, nil
}
