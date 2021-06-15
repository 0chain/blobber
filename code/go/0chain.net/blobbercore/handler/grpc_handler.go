package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/convert"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

type blobberGRPCService struct {
	storageHandler StorageHandlerI
	packageHandler PackageHandler
	blobbergrpc.UnimplementedBlobberServer
}

func newGRPCBlobberService(sh StorageHandlerI, r PackageHandler) *blobberGRPCService {
	return &blobberGRPCService{
		storageHandler: sh,
		packageHandler: r,
	}
}

func (b *blobberGRPCService) GetAllocation(ctx context.Context, request *blobbergrpc.GetAllocationRequest) (*blobbergrpc.GetAllocationResponse, error) {
	r, err := http.NewRequest("GET", "", nil)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, GetGRPCMetaDataFromCtx(ctx), "")
	r.Form = map[string][]string{"id": {request.Id}}

	resp, err := AllocationHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.GetAllocationResponseCreator(resp), nil
}

func (b *blobberGRPCService) GetFileMetaData(ctx context.Context, req *blobbergrpc.GetFileMetaDataRequest) (*blobbergrpc.GetFileMetaDataResponse, error) {
	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, GetGRPCMetaDataFromCtx(ctx), req.Allocation)
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
	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, GetGRPCMetaDataFromCtx(ctx), req.Allocation)
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

func (b *blobberGRPCService) ListEntities(ctx context.Context, req *blobbergrpc.ListEntitiesRequest) (*blobbergrpc.ListEntitiesResponse, error) {
	r, err := http.NewRequest("", "", nil)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, GetGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path":       {req.Path},
		"path_hash":  {req.PathHash},
		"auth_token": {req.AuthToken},
	}

	resp, err := ListHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.ListEntitesResponseCreator(resp), nil
}

func (b *blobberGRPCService) GetObjectPath(ctx context.Context, req *blobbergrpc.GetObjectPathRequest) (*blobbergrpc.GetObjectPathResponse, error) {
	r, err := http.NewRequest("", "", nil)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, GetGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path":      {req.Path},
		"block_num": {req.BlockNum},
	}

	resp, err := ObjectPathHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.GetObjectPathResponseCreator(resp), nil
}

func (b *blobberGRPCService) GetReferencePath(ctx context.Context, req *blobbergrpc.GetReferencePathRequest) (*blobbergrpc.GetReferencePathResponse, error) {
	md := GetGRPCMetaDataFromCtx(ctx)
	allocationTx := req.Allocation
	allocationObj, err := b.storageHandler.verifyAllocation(ctx, allocationTx, false)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}
	allocationID := allocationObj.ID

	valid, err := verifySignatureFromRequest(allocationTx, md.ClientSignature, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	clientID := md.Client
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

	refPath := &reference.ReferencePath{Ref: rootRef}
	refsToProcess := make([]*reference.ReferencePath, 0)
	refsToProcess = append(refsToProcess, refPath)
	for len(refsToProcess) > 0 {
		refToProcess := refsToProcess[0]
		refToProcess.Meta = refToProcess.Ref.GetListingData(ctx)
		if len(refToProcess.Ref.Children) > 0 {
			refToProcess.List = make([]*reference.ReferencePath, len(refToProcess.Ref.Children))
		}
		for idx, child := range refToProcess.Ref.Children {
			childRefPath := &reference.ReferencePath{Ref: child}
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
	var recursionCount int
	refPathResult.ReferencePath = convert.ReferencePathToReferencePathGRPC(&recursionCount, refPath)
	if latestWM != nil {
		refPathResult.LatestWM = convert.WriteMarkerToWriteMarkerGRPC(&latestWM.WM)
	}

	return &refPathResult, nil
}

func (b *blobberGRPCService) GetObjectTree(ctx context.Context, req *blobbergrpc.GetObjectTreeRequest) (*blobbergrpc.GetObjectTreeResponse, error) {
	allocationTx := req.Allocation
	allocationObj, err := b.storageHandler.verifyAllocation(ctx, allocationTx, false)
	md := GetGRPCMetaDataFromCtx(ctx)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}
	allocationID := allocationObj.ID

	valid, err := verifySignatureFromRequest(allocationTx, md.ClientSignature, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	clientID := md.Client
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

	refPath := &reference.ReferencePath{Ref: rootRef}
	refsToProcess := make([]*reference.ReferencePath, 0)
	refsToProcess = append(refsToProcess, refPath)
	for len(refsToProcess) > 0 {
		refToProcess := refsToProcess[0]
		refToProcess.Meta = refToProcess.Ref.GetListingData(ctx)
		if len(refToProcess.Ref.Children) > 0 {
			refToProcess.List = make([]*reference.ReferencePath, len(refToProcess.Ref.Children))
		}
		for idx, child := range refToProcess.Ref.Children {
			childRefPath := &reference.ReferencePath{Ref: child}
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
	var recursionCount int
	refPathResult.ReferencePath = convert.ReferencePathToReferencePathGRPC(&recursionCount, refPath)
	if latestWM != nil {
		refPathResult.LatestWM = convert.WriteMarkerToWriteMarkerGRPC(&latestWM.WM)
	}
	return &refPathResult, nil
}

func (b *blobberGRPCService) CalculateHash(ctx context.Context, req *blobbergrpc.CalculateHashRequest) (*blobbergrpc.CalculateHashResponse, error) {
	allocationTx := req.Allocation
	allocationObj, err := b.storageHandler.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	md := GetGRPCMetaDataFromCtx(ctx)
	allocationID := allocationObj.ID

	clientID := md.Client
	if len(clientID) == 0 || allocationObj.OwnerID != clientID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
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

	if _, err := rootRef.CalculateHash(ctx, true); err != nil {
		return nil, err
	}

	return &blobbergrpc.CalculateHashResponse{Message: "Hash recalculated for the given paths"}, nil
}

func (b *blobberGRPCService) CommitMetaTxn(ctx context.Context, req *blobbergrpc.CommitMetaTxnRequest) (*blobbergrpc.CommitMetaTxnResponse, error) {
	allocationTx := req.GetAllocation()
	allocationObj, err := b.storageHandler.verifyAllocation(ctx, allocationTx, true)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}
	allocationID := allocationObj.ID

	md := GetGRPCMetaDataFromCtx(ctx)
	clientID := md.Client
	if len(clientID) == 0 {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	pathHash := req.PathHash
	path := req.Path
	if len(pathHash) == 0 {
		if len(path) == 0 {
			return nil, common.NewError("invalid_parameters", "Invalid path")
		}
		pathHash = reference.GetReferenceLookup(allocationID, path)
	}

	fileRef, err := b.packageHandler.GetReferenceFromLookupHash(ctx, allocationID, pathHash)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	if fileRef.Type != reference.FILE {
		return nil, common.NewError("invalid_parameters", "Path is not a file.")
	}

	auhToken := req.GetAuthToken()

	if clientID != allocationObj.OwnerID || len(auhToken) > 0 {
		authTicketVerified, err := b.storageHandler.verifyAuthTicket(ctx, auhToken, allocationObj, fileRef, clientID)
		if err != nil {
			return nil, err
		}

		if !authTicketVerified {
			return nil, common.NewError("auth_ticket_verification_failed", "Could not verify the auth ticket.")
		}
	}

	txnID := req.GetTxnId()
	if len(txnID) == 0 {
		return nil, common.NewError("invalid_parameter", "TxnID not present in the params")
	}

	if err := b.packageHandler.AddCommitMetaTxn(ctx, fileRef.ID, txnID); err != nil {
		return nil, common.NewError("add_commit_meta_txn_failed", "Failed to add commitMetaTxn with err :"+err.Error())
	}

	return &blobbergrpc.CommitMetaTxnResponse{
		Message: "Added commitMetaTxn successfully",
	}, nil
}

func (b *blobberGRPCService) Collaborator(ctx context.Context, req *blobbergrpc.CollaboratorRequest) (*blobbergrpc.CollaboratorResponse, error) {
	allocationTx := req.Allocation
	allocationObj, err := b.storageHandler.verifyAllocation(ctx, allocationTx, true)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	md := GetGRPCMetaDataFromCtx(ctx)
	allocationID := allocationObj.ID

	valid, err := verifySignatureFromRequest(allocationTx, md.ClientSignature, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	clientID := md.Client

	pathHash := req.PathHash
	path := req.Path
	if len(pathHash) == 0 {
		if len(path) == 0 {
			return nil, common.NewError("invalid_parameters", "Invalid path")
		}
		pathHash = reference.GetReferenceLookup(allocationID, path)
	}

	fileRef, err := b.packageHandler.GetReferenceFromLookupHash(ctx, allocationID, pathHash)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	if fileRef.Type != reference.FILE {
		return nil, common.NewError("invalid_parameters", "Path is not a file.")
	}

	collabClientID := req.CollabId
	if len(collabClientID) == 0 {
		return nil, common.NewError("invalid_parameter", "collab_id not present in the params")
	}

	var msg string

	switch req.GetMethod() {
	case http.MethodPost:
		if len(clientID) == 0 || clientID != allocationObj.OwnerID {
			return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
		}

		if b.packageHandler.IsACollaborator(ctx, fileRef.ID, collabClientID) {
			msg = "Given client ID is already a collaborator"
			return &blobbergrpc.CollaboratorResponse{Message: msg}, nil
		}

		if err := b.packageHandler.AddCollaborator(ctx, fileRef.ID, collabClientID); err != nil {
			return nil, common.NewError("add_collaborator_failed", "Failed to add collaborator with err :"+err.Error())
		}

		msg = "Added collaborator successfully"

	case http.MethodGet:
		collaborators, err := b.packageHandler.GetCollaborators(ctx, fileRef.ID)
		if err != nil {
			return nil, common.NewError("get_collaborator_failed", "Failed to get collaborators from refID with err:"+err.Error())
		}

		var collaboratorsGRPC []*blobbergrpc.Collaborator
		for _, c := range collaborators {
			collaboratorsGRPC = append(collaboratorsGRPC, convert.CollaboratorToGRPCCollaborator(&c))
		}

		return &blobbergrpc.CollaboratorResponse{
			Collaborators: collaboratorsGRPC,
		}, nil

	case http.MethodDelete:
		if len(clientID) == 0 || clientID != allocationObj.OwnerID {
			return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
		}

		if err := b.packageHandler.RemoveCollaborator(ctx, fileRef.ID, collabClientID); err != nil {
			return nil, common.NewError("delete_collaborator_failed", "Failed to delete collaborator from refID with err:"+err.Error())
		}

		msg = "Removed collaborator successfully"

	default:
		return nil, common.NewError("invalid_method", "Invalid method used. Use POST/GET/DELETE instead")
	}

	return &blobbergrpc.CollaboratorResponse{
		Message: msg,
	}, nil
}
