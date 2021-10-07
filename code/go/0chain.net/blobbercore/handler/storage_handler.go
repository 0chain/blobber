package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/0chain/gosdk/zboxcore/fileref"
	"gorm.io/gorm"
	"net/http"
	"strconv"
	"strings"

	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"github.com/pkg/errors"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/constants"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

const (
	FORM_FILE_PARSE_MAX_MEMORY = 10 * 1024 * 1024

	DOWNLOAD_CONTENT_FULL  = "full"
	DOWNLOAD_CONTENT_THUMB = "thumbnail"
)

type StorageHandler struct{}

func (fsh *StorageHandler) verifyAllocation(ctx context.Context, tx string,
	readonly bool) (alloc *allocation.Allocation, err error) {

	if len(tx) == 0 {
		return nil, common.NewError("verify_allocation",
			"invalid allocation id")
	}

	logging.Logger.Info("call allocation.VerifyAllocationTransaction",
		zap.String("tx", tx))
	alloc, err = allocation.VerifyAllocationTransaction(ctx, tx, readonly)
	if err != nil {
		return nil, common.NewErrorf("verify_allocation",
			"verifying allocation transaction error: %v", err)
	}

	if alloc.Expiration < common.Now() {
		return nil, common.NewError("verify_allocation",
			"use of expired allocation")
	}

	return
}

func (fsh *StorageHandler) verifyAuthTicket(ctx context.Context, authTokenString string, allocationObj *allocation.Allocation, refRequested *reference.Ref, clientID string) (bool, error) {
	if len(authTokenString) == 0 {
		return false, common.NewError("invalid_parameters", "Auth ticket required if data read by anyone other than owner.")
	}
	authToken := &readmarker.AuthTicket{}
	err := json.Unmarshal([]byte(authTokenString), &authToken)
	if err != nil {
		return false, common.NewError("invalid_parameters", "Error parsing the auth ticket for download."+err.Error())
	}
	err = authToken.Verify(allocationObj, clientID)
	if err != nil {
		return false, err
	}
	if refRequested.LookupHash != authToken.FilePathHash {
		authTokenRef, err := reference.GetReferenceFromLookupHash(ctx, authToken.AllocationID, authToken.FilePathHash)
		if err != nil {
			return false, err
		}
		if refRequested.ParentPath != authTokenRef.Path && !strings.HasPrefix(refRequested.ParentPath, authTokenRef.Path+"/") {
			return false, common.NewError("invalid_parameters", "Auth ticket is not valid for the resource being requested")
		}
	}

	return true, nil
}

func (fsh *StorageHandler) GetAllocationDetails(ctx context.Context, request *blobbergrpc.GetAllocationRequest) (interface{}, error) {
	allocationObj, err := fsh.verifyAllocation(ctx, request.Id, false)

	if err != nil {
		return nil, errors.Wrap(err, "unable to GetAllocationDetails for id: "+request.Id)
	}

	return allocationObj, nil
}

func (fsh *StorageHandler) GetAllocationUpdateTicket(ctx context.Context, r *http.Request) (interface{}, error) {
	if r.Method != "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use GET instead")
	}
	allocationTx := r.FormValue("id")
	allocationObj, err := fsh.verifyAllocation(ctx, allocationTx, false)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	// TODO

	return allocationObj, nil
}

func (fsh *StorageHandler) checkIfFileAlreadyExists(ctx context.Context, allocationID string, path string) *reference.Ref {
	fileReference, err := reference.GetReference(ctx, allocationID, path)
	if err != nil {
		return nil
	}
	return fileReference
}

func (fsh *StorageHandler) GetFileMeta(ctx context.Context, request *blobbergrpc.GetFileMetaDataRequest) (interface{}, error) {

	allocationTx := request.Allocation
	alloc, err := fsh.verifyAllocation(ctx, allocationTx, true)

	if err != nil {
		Logger.Error("Invalid allocation ID passed in the request")
		return nil, errors.Wrap(err, "invalid allocation id: "+allocationTx)
	}

	allocationID := alloc.ID
	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)

	if clientID == "" {
		Logger.Error("Operation needs to be performed by the owner of allocation")
		return nil, errors.Wrap(errors.New("missing client id"), "Operation needs to be performed by the owner of the allocation")
	}

	if request.PathHash == "" {
		if request.Path == "" {
			Logger.Error("Invalid request path passed in the request")
			return nil, errors.Wrapf(errors.New("invalid request parameters"), "invalid request path")
		}
		request.PathHash = reference.GetReferenceLookup(allocationID, request.Path)
	}

	fileRef, err := reference.GetReferenceFromLookupHash(ctx, allocationID, request.PathHash)
	if err != nil {
		Logger.Error("Invalid file path passed in the request")
		return nil, errors.Wrap(err, "invalid file path passed in the request")
	}

	result := fileRef.GetListingData(ctx)
	commitMetaTxns, err := reference.GetCommitMetaTxns(ctx, fileRef.ID)

	if err != nil {
		Logger.Error("Failed to get commitMetaTxns from refID", zap.Error(err), zap.Any("ref_id", fileRef.ID))
		return nil, errors.Wrapf(err, "failed to get commitMetaTxns from refID: %v", fileRef.ID)
	}

	result["commit_meta_txns"] = commitMetaTxns

	collaborators, err := reference.GetCollaborators(ctx, fileRef.ID)
	if err != nil {
		Logger.Error("Failed to get collaborators from refID", zap.Error(err), zap.Any("ref_id", fileRef.ID))
		return nil, errors.Wrapf(err, "failed to get collaborators from refID: %v", fileRef.ID)
	}

	result["collaborators"] = collaborators

	if !(clientID == alloc.OwnerID) && !(clientID == alloc.RepairerID) && !reference.IsACollaborator(ctx, fileRef.ID, clientID) {
		// check if auth ticket is valid or not
		if isAuthorized, err := fsh.verifyAuthTicket(ctx,
			request.AuthToken, alloc, fileRef, clientID,
		); !isAuthorized {
			Logger.Error("Failed to verify the authorisation ticket")
			return nil, errors.Wrap(err, "failed to verify the auth ticket")
		}
		delete(result, "path")
	}

	return result, nil
}

func (fsh *StorageHandler) AddCommitMetaTxn(ctx context.Context, request *blobbergrpc.CommitMetaTxnRequest) (*blobbergrpc.CommitMetaTxnResponse, error) {

	// todo(kushthedude): generalise the allocation_context in the grpc metadata
	//allocationTx := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, request.Allocation, true)

	if err != nil {
		return nil, errors.Wrap(err,
			"invalid allocation id passed in the request")
	}

	allocationID := allocationObj.ID

	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	if clientID == "" && clientID == allocationObj.OwnerID {
		return nil, errors.Wrap(errors.New("Authorisation Error"),
			"operation can be performed by owner of allocation")
	}

	if request.PathHash == "" {
		if request.Path == "" {
			Logger.Error("Invalid request path passed in the request")
			return nil, errors.Wrapf(errors.New("invalid request parameters"), "invalid request path")
		}
		request.PathHash = reference.GetReferenceLookup(allocationID, request.Path)
	}

	fileref, err := reference.GetReferenceFromLookupHash(ctx, allocationID, request.PathHash)
	if err != nil && fileref.Type != reference.FILE {
		return nil, errors.Wrap(errors.New("Invalid Parameters"),
			"failed to fetch File from file path")
	}

	if clientID != allocationObj.OwnerID || request.AuthToken != "" {
		authTicketVerified, err := fsh.verifyAuthTicket(ctx, request.AuthToken, allocationObj, fileref, clientID)
		if err != nil && !authTicketVerified {
			return nil, errors.Wrap(errors.New("Authorisation Error"),
				"failed to verify AuthTicket")
		}
	}

	if request.TxnId == "" {
		return nil, errors.Wrap(errors.New("Parameter Error"),
			"transaction ID cant be empty")
	}

	err = reference.AddCommitMetaTxn(ctx, fileref.ID, request.TxnId)
	if err != nil {
		return nil, errors.Wrap(err,
			"failed to add commitMetaTxn")
	}

	var result blobbergrpc.CommitMetaTxnResponse
	result.Message = "Added commitMetaTxn successfully"

	return &result, nil
}

func (fsh *StorageHandler) AddCollaborator(ctx context.Context, request *blobbergrpc.CollaboratorRequest) (*blobbergrpc.CollaboratorResponse, error) {
	//allocationTx := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, request.Allocation, true)
	if err != nil {
		return nil, errors.Wrap(err, "invalid allocation id passed")
	}

	clientSign := ctx.Value(constants.CLIENT_SIGNATURE_HEADER_KEY).(string)
	valid, err := verifySignatureFromRequest(request.Allocation, clientSign, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, errors.Wrap(invalidParameters, "invalid signature passed")
	}

	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)

	if request.PathHash == "" {
		if request.Path == "" {
			Logger.Error("Invalid request path passed in the request")
			return nil, errors.Wrapf(errors.New("invalid request parameters"), "invalid request path")
		}
		request.PathHash = reference.GetReferenceLookup(allocationObj.ID, request.Path)
	}

	fileref, err := reference.GetReferenceFromLookupHash(ctx, allocationObj.ID, request.PathHash)
	if err != nil {
		return nil, errors.Wrap(err, "invalid file path")
	}

	if fileref.Type != reference.FILE {
		return nil, errors.Wrap(invalidParameters, "path is not a filePath")
	}

	if request.CollabId == "" {
		return nil, errors.Wrap(invalidParameters, "collab_id is missing in request")
	}

	var response blobbergrpc.CollaboratorResponse

	switch request.Method {
	case http.MethodPost:
		if clientID == "" || clientID != allocationObj.OwnerID {
			return nil, errors.Wrap(authorisationError, "operation needs to be performed by owner")
		}

		if reference.IsACollaborator(ctx, fileref.ID, request.CollabId) {
			response.Message = "Given client ID is already a collaborator"
			return &response, nil
		}

		err = reference.AddCollaborator(ctx, fileref.ID, request.CollabId)
		if err != nil {
			return nil, common.NewError("add_collaborator_failed", "Failed to add collaborator with err :"+err.Error())
		}
		response.Message = "Added collaborator successfully"

	case http.MethodGet:
		collaborators, err := reference.GetCollaborators(ctx, fileref.ID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get the collaborator")
		}
		for _, c := range collaborators {
			response.Collaborators = append(response.Collaborators, &blobbergrpc.Collaborator{
				RefId:     c.RefID,
				ClientId:  c.ClientID,
				CreatedAt: c.CreatedAt.UnixNano(),
			})
		}

		return &response, nil

	case http.MethodDelete:
		if clientID == "" || clientID != allocationObj.OwnerID {
			return nil, errors.Wrap(authorisationError, "operation needs to be performed by owner")
		}

		err = reference.RemoveCollaborator(ctx, fileref.ID, request.CollabId)
		if err != nil {
			return nil, errors.Wrap(err, "failed to delete the collaborator")
		}
		response.Message = "Removed collaborator successfully"

	default:
		return nil, errors.Wrap(errors.New("Invalid Method"), "use GET/POST/DELETE only")
	}

	return &response, nil
}

func (fsh *StorageHandler) GetFileStats(ctx context.Context, request *blobbergrpc.GetFileStatsRequest) (interface{}, error) {
	// todo(kushthedude): generalise the allocation_context in the grpc metadata
	alloc, err := fsh.verifyAllocation(ctx, request.Allocation, true)
	if err != nil {
		Logger.Error("Invalid allocation ID passed in the request")
		return nil, errors.Wrap(err, "invalid allocation id passed")
	}

	allocationID := alloc.ID
	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	if clientID == "" {
		Logger.Error("Operation needs to be performed by the owner of allocation")
		return nil, errors.Wrap(errors.New("missing client id"), "Operation needs to be performed by the owner of the allocation")
	}

	clientSign := ctx.Value(constants.CLIENT_SIGNATURE_HEADER_KEY).(string)
	valid, err := verifySignatureFromRequest(request.Allocation, clientSign, alloc.OwnerPublicKey)
	if !valid || err != nil {
		return nil, errors.Wrap(invalidParameters, "invalid signature passed")
	}

	if request.PathHash == "" {
		if request.Path == "" {
			Logger.Error("Invalid request path passed in the request")
			return nil, errors.Wrapf(errors.New("invalid request parameters"), "invalid request path")
		}
		request.PathHash = reference.GetReferenceLookup(allocationID, request.Path)
	}

	fileref, err := reference.GetReferenceFromLookupHash(ctx, allocationID, request.PathHash)
	if err != nil {
		return nil, errors.Wrap(err, "invalid file path")
	}
	if fileref.Type != reference.FILE {
		return nil, errors.Wrap(invalidParameters, "path is not a filePath")
	}

	result := fileref.GetListingData(ctx)
	fileStats, err := stats.GetFileStats(ctx, fileref.ID)
	if err != nil {
		Logger.Error("unable to get file stats from fileRef ", zap.Int64("fileRef.id", fileref.ID))
		Logger.Error(err.Error(), zap.Int64("fileRef.id", fileref.ID)) // for debug
		//return nil, errors.Wrap(err, "failed to get fileStats from the fileRef")
	}

	wm, err := writemarker.GetWriteMarkerEntity(ctx, fileref.WriteMarker)
	if err != nil {
		Logger.Error("unable to get write marker from fileRef ", zap.String("fileRef.WriteMarker", fileref.WriteMarker))
		Logger.Error(err.Error(), zap.Int64("fileRef.id", fileref.ID)) // for debug
		//	return nil, errors.Wrap(err, "failed to get write marker from fileRef")
	}
	if wm != nil && fileStats != nil {
		fileStats.WriteMarkerRedeemTxn = wm.CloseTxnID
	}

	var statsMap map[string]interface{}

	statsBytes, err := json.Marshal(fileStats)
	if err != nil {
		Logger.Error("unable to marshal fileStats ")
		return nil, errors.Wrapf(err, "failed to marshal fileStats")
	}
	if err = json.Unmarshal(statsBytes, &statsMap); err != nil {
		Logger.Error("unable to unmarshal into statsMap ")
		return nil, errors.Wrapf(err, "failed to marshal into statsMap")
	}

	for k, v := range statsMap {
		result[k] = v
	}

	return result, nil
}

func (fsh *StorageHandler) ListEntities(ctx context.Context, request *blobbergrpc.ListEntitiesRequest) (*blobberhttp.ListResult, error) {

	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	// todo(kushthedude): generalise the allocation_context in the grpc metadata
	//allocationTx := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, request.Allocation, true)

	if err != nil {
		Logger.Error("Invalid allocation id passed")
		return nil, errors.Wrap(err, "invalid allocation id passed")
	}
	allocationID := allocationObj.ID

	if len(clientID) == 0 {
		Logger.Error("Operation needs to be performed by the owner of the allocation")
		return nil, errors.Wrap(errors.New("Unauthorised Operation"), "operation needs to be performed by the owner of the allocation")
	}

	if request.PathHash == "" {
		if request.Path == "" {
			Logger.Error("Invalid request path passed in the request")
			return nil, errors.Wrapf(errors.New("invalid request parameters"), "invalid request path")
		}
		request.PathHash = reference.GetReferenceLookup(allocationID, request.Path)
	}

	Logger.Debug("Path Hash for list dir :" + request.PathHash)

	fileref, err := reference.GetReferenceFromLookupHash(ctx, allocationID, request.PathHash)
	if err != nil {
		Logger.Error("Invalid request pathHash passed in the request")
		return nil, errors.Wrapf(err, "invalid request path has")
	}

	if clientID != allocationObj.OwnerID || request.AuthToken != "" {
		authTicketVerified, err := fsh.verifyAuthTicket(ctx, request.AuthToken, allocationObj, fileref, clientID)
		if err != nil || !authTicketVerified {
			Logger.Error("Unable to verify authTicket")
			return nil, errors.Wrapf(errors.New("Unauthorised Operation"), "failed to verify authTicker")
		}
	}

	dirref, err := reference.GetRefWithChildren(ctx, allocationID, fileref.Path)
	if err != nil {
		Logger.Error("Invalid fileRef Path passed in the request", zap.String("fileRef", fileref.Path))
		return nil, errors.Wrapf(err, "invalid fileRef path has")
	}

	var result blobberhttp.ListResult
	result.AllocationRoot = allocationObj.AllocationRoot
	result.Meta = dirref.GetListingData(ctx)
	if clientID != allocationObj.OwnerID {
		delete(result.Meta, "path")
	}
	result.Entities = make([]map[string]interface{}, len(dirref.Children))
	for idx, child := range dirref.Children {
		result.Entities[idx] = child.GetListingData(ctx)
		if clientID != allocationObj.OwnerID {
			delete(result.Entities[idx], "path")
		}
	}

	return &result, nil
}

func (fsh *StorageHandler) GetReferencePath(ctx context.Context, request *blobbergrpc.GetReferencePathRequest) (*blobberhttp.ReferencePathResult, error) {

	// todo(kushthedude): generalise the allocation_context in the grpc metadata
	//allocationTx := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)

	logging.Logger.Info("call fsh.verifyAllocation",
		zap.String("allocation", request.Allocation))
	allocationObj, err := fsh.verifyAllocation(ctx, request.Allocation, false)
	if err != nil {
		return nil, errors.Wrap(errors.New("Invalid Request"), "Invalid allocation ID passed")
	}

	allocationID := allocationObj.ID

	clientSign := ctx.Value(constants.CLIENT_SIGNATURE_HEADER_KEY).(string)
	valid, err := verifySignatureFromRequest(request.Allocation, clientSign, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, errors.Wrap(errors.New("Invalid Request"), "Invalid signature passed")
	}

	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	if clientID == "" {
		return nil, errors.Wrap(errors.New("Invalid Request"), "Invalid client ID passed")
	}

	paths, err := pathsFromGrpcRequest(request.Paths, request.Path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get paths from req")
	}

	rootRef, err := reference.GetReferencePathFromPaths(ctx, allocationID, paths)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get reference path from req")
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
			return nil, errors.Wrap(err, "failed to read latest write marker allocation")
		}
	}

	var refPathResult blobberhttp.ReferencePathResult
	refPathResult.ReferencePath = refPath
	if latestWM != nil {
		refPathResult.LatestWM = &latestWM.WM
	}

	return &refPathResult, nil
}

func (fsh *StorageHandler) GetObjectPath(ctx context.Context, request *blobbergrpc.GetObjectPathRequest) (*blobberhttp.ObjectPathResult, error) {

	//todo:kushthedude replace request.Allocation with the ctx stuff
	allocationObj, err := fsh.verifyAllocation(ctx, request.Allocation, false)
	if err != nil {
		return nil, errors.Wrap(err, "invalid allocation id in request")
	}
	allocationID := allocationObj.ID

	clientSign, _ := ctx.Value(constants.CLIENT_SIGNATURE_HEADER_KEY).(string)
	valid, err := verifySignatureFromRequest(request.Allocation, clientSign, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, errors.Wrap(errors.New("Invalid Parameter"), "invalid signature header in request")
	}

	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	if len(clientID) == 0 || allocationObj.OwnerID != clientID {
		return nil, errors.Wrap(errors.New("Authorisation Error"), "operation needs to be performed by the owner of the allocation")
	}

	if request.Path == "" && request.BlockNum == "" {
		return nil, errors.Wrap(errors.New("Invalid Parameter"), "invalid path in request")
	}

	blockNum, err := strconv.ParseInt(request.BlockNum, 10, 64)
	if err != nil || blockNum < 0 {
		return nil, errors.Wrap(errors.New("Invalid Parameter"), "invalid blockNum in request")
	}

	objectPath, err := reference.GetObjectPath(ctx, allocationID, blockNum)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get object path")
	}

	var latestWM *writemarker.WriteMarkerEntity
	if len(allocationObj.AllocationRoot) == 0 {
		latestWM = nil
	} else {
		latestWM, err = writemarker.GetWriteMarkerEntity(ctx, allocationObj.AllocationRoot)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read latest write marker for allocation")
		}
	}
	var objPathResult blobberhttp.ObjectPathResult
	objPathResult.ObjectPath = objectPath
	if latestWM != nil {
		objPathResult.LatestWM = &latestWM.WM
	}
	return &objPathResult, nil
}

func (fsh *StorageHandler) GetObjectTree(ctx context.Context, request *blobbergrpc.GetObjectTreeRequest) (*blobberhttp.ReferencePathResult, error) {

	// todo(kushthedude): generalise the allocation_context in the grpc metadata
	//allocationTx := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, request.Allocation, false)

	if err != nil {
		return nil, errors.Wrap(err, "failed to verify allocation")
	}

	allocationID := allocationObj.ID

	clientSign := ctx.Value(constants.CLIENT_SIGNATURE_HEADER_KEY).(string)
	valid, err := verifySignatureFromRequest(request.Allocation, clientSign, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, errors.Wrap(errors.New("Authorisation Error"), "failed to verify signature")
	}

	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	if len(clientID) == 0 || allocationObj.OwnerID != clientID {
		return nil, errors.Wrap(errors.New("Authorisation Error"),
			"operation needs to be performed by the owner of the allocation")
	}

	if request.Path == "" {
		return nil, errors.Wrap(errors.New("Invalid Parameters"),
			"invalid path passed in request")
	}

	rootRef, err := reference.GetObjectTree(ctx, allocationID, request.Path)
	if err != nil {
		return nil, errors.Wrap(err,
			"failed to get object tree from allocation id")
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
			return nil, errors.Wrap(err,
				"failed to read the latest write marker for allocation")
		}
	}
	var refPathResult blobberhttp.ReferencePathResult
	refPathResult.ReferencePath = refPath
	if latestWM != nil {
		refPathResult.LatestWM = &latestWM.WM
	}
	return &refPathResult, nil
}

func (fsh *StorageHandler) CalculateHash(ctx context.Context, request *blobbergrpc.CalculateHashRequest) (*blobbergrpc.CalculateHashResponse, error) {
	//if r.Method != "POST" {
	//	return nil, common.NewError("invalid_method", "Invalid method used. Use POST instead")
	//}
	// todo(kushthedude): generalise the allocation_context in the grpc metadata
	//allocationTx := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, request.Allocation, false)

	if err != nil {
		return nil, errors.Wrap(err,
			"invalid allocation ID passed in request")
	}
	allocationID := allocationObj.ID

	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	if clientID == "" || allocationObj.OwnerID != clientID {
		return nil, errors.Wrap(errors.New("Authorisation Error"),
			"operation can be performed by owner of the allocation")
	}

	paths, err := pathsFromGrpcRequest(request.Paths, request.Path)
	if err != nil {
		return nil, errors.Wrap(err,
			"invalid path passed in request")
	}

	rootRef, err := reference.GetReferencePathFromPaths(ctx, allocationID, paths)
	if err != nil {
		return nil, errors.Wrap(err,
			"failed to get reference path from Paths")
	}

	if _, err := rootRef.CalculateHash(ctx, true); err != nil {
		return nil, errors.Wrap(err,
			"failed to calculate hash for the rootRef")
	}

	var result blobbergrpc.CalculateHashResponse
	result.Message = "Hash recalculated for the given paths"

	return &result, nil
}

func (fsh *StorageHandler) MarketplaceShareInfo(ctx context.Context, request *blobbergrpc.MarketplaceShareInfoRequest) (*blobbergrpc.MarketplaceShareInfoResponse, error) {
	switch request.HttpMethod {
	case http.MethodDelete:
		return fsh.RevokeShare(ctx, request)
	case http.MethodPost:
		return fsh.InsertShare(ctx, request)
	default:
		return nil, errors.Wrap(errors.New("Method not allowed"), "POST/DELETE are allowed for httpMethod")
	}
}

func (fsh *StorageHandler) RevokeShare(ctx context.Context, request *blobbergrpc.MarketplaceShareInfoRequest) (*blobbergrpc.MarketplaceShareInfoResponse, error) {
	ctx = setupGrpcHandlerContext(ctx, getGRPCMetaDataFromCtx(ctx))

	allocationObj, err := fsh.verifyAllocation(ctx, request.Allocation, true)
	if err != nil {
		return nil, errors.Wrap(err, "invalid allocation ID passed.")
	}

	sign := ctx.Value(constants.CLIENT_SIGNATURE_HEADER_KEY).(string)
	valid, err := verifySignatureFromRequest(request.Allocation, sign, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, errors.Wrap(err, "Invalid signature")
	}

	filePathHash := fileref.GetReferenceLookup(allocationObj.ID, request.Path)
	_, err = reference.GetReferenceFromLookupHash(ctx, allocationObj.ID, filePathHash)
	if err != nil {
		return nil, errors.Wrap(err, "Invalid file path")
	}

	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	if clientID != allocationObj.OwnerID {
		return nil, errors.Wrap(err, "Operation needs to be performed by the owner of the allocation")
	}
	err = reference.DeleteShareInfo(ctx, reference.ShareInfo{
		ClientID:     request.RefereeClientId,
		FilePathHash: filePathHash,
	})

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// todo: NOT_FOUND grpc error code
		return &blobbergrpc.MarketplaceShareInfoResponse{
			StatusCode: http.StatusNotFound,
			Message:    "Path not found",
		}, nil
	}
	if err != nil {
		return nil, err
	}

	return &blobbergrpc.MarketplaceShareInfoResponse{
		StatusCode: http.StatusNoContent,
		Message:    "Path successfully removed from allocation",
	}, nil
}

func (fsh *StorageHandler) InsertShare(ctx context.Context, request *blobbergrpc.MarketplaceShareInfoRequest) (*blobbergrpc.MarketplaceShareInfoResponse, error) {
	ctx = setupGrpcHandlerContext(ctx, getGRPCMetaDataFromCtx(ctx))

	allocationObj, err := fsh.verifyAllocation(ctx, request.Allocation, true)
	if err != nil {
		return nil, errors.Wrap(err, "invalid allocation ID passed.")
	}

	sign := ctx.Value(constants.CLIENT_SIGNATURE_HEADER_KEY).(string)
	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	valid, err := verifySignatureFromRequest(request.Allocation, sign, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, errors.Wrap(err, "Invalid signature")
	}

	if request.Path == "" {
		Logger.Error("Invalid request path passed in the request")
		return nil, errors.Wrapf(errors.New("invalid request parameters"), "invalid request path")
	}
	pathHash := reference.GetReferenceLookup(allocationObj.ID, request.Path)
	fmt.Printf("pathhash is %v \n", pathHash)

	fileReference, err := reference.GetReferenceFromLookupHash(ctx, allocationObj.ID, pathHash)
	if err != nil {
		return nil, errors.Wrap(err, "Invalid file path")
	}

	if clientID != allocationObj.OwnerID || request.AuthTicket != "" {
		authTicketVerified, err := fsh.verifyAuthTicket(ctx, request.AuthTicket, allocationObj, fileReference, clientID)
		if err != nil && !authTicketVerified {
			return nil, errors.Wrap(errors.New("Authorisation Error"),
				"failed to verify AuthTicket")
		}
	}

	shareInfo := reference.ShareInfo{
		OwnerID:                   allocationObj.OwnerID,
		ClientID:                  clientID,
		FilePathHash:              pathHash,
		ReEncryptionKey:           allocationObj.OwnerPublicKey,
		ClientEncryptionPublicKey: request.EncryptionPublicKey,
		ExpiryAt:                  common.ToTime(allocationObj.Expiration),
	}

	existingShare, err := reference.GetShareInfo(ctx, clientID, pathHash)
	if err != nil {
		return nil, errors.Wrap(err, "error getting share info")
	}

	if existingShare != nil {
		err = reference.UpdateShareInfo(ctx, shareInfo)
	} else {
		err = reference.AddShareInfo(ctx, shareInfo)
	}
	if err != nil {
		return nil, errors.Wrap(err, "error updating/adding share")
	}

	resp := &blobbergrpc.MarketplaceShareInfoResponse{
		StatusCode: http.StatusOK,
		Message:    "Share info added successfully",
	}

	return resp, nil
}

// verifySignatureFromRequest verifies signature passed as common.ClientSignatureHeader header.
func verifySignatureFromRequest(allocation, sign, pbK string) (bool, error) {
	sign = encryption.MiraclToHerumiSig(sign)

	if len(sign) < 64 {
		return false, nil
	}

	hash := encryption.Hash(allocation)
	return encryption.Verify(pbK, sign, hash)
}

// pathsFromReq retrieves paths value from request which can be represented as single "path" value or "paths" values,
// marshalled to json.
//func pathsFromReq(r *http.Request) ([]string, error) {
//	var (
//		pathsStr = r.FormValue("paths")
//		path     = r.FormValue("path")
//		paths    = make([]string, 0)
//	)
//
//	if len(pathsStr) == 0 {
//		if len(path) == 0 {
//			return nil, common.NewError("invalid_parameters", "Invalid path")
//		}
//
//		return append(paths, path), nil
//	}
//
//	if err := json.Unmarshal([]byte(pathsStr), &paths); err != nil {
//		return nil, common.NewError("invalid_parameters", "Invalid path array json")
//	}
//
//	return paths, nil
//}

func pathsFromGrpcRequest(paths string, path string) ([]string, error) {
	pathsArr := make([]string, 0)
	if paths == "" {
		if path == "" {
			return nil, errors.Wrap(errors.New("Invalid Parameters"),
				"invalid path passed in request")
		}
		return append(pathsArr, path), nil
	}

	if err := json.Unmarshal([]byte(paths), &pathsArr); err != nil {
		return nil, errors.Wrap(errors.New("Invalid Parameters"),
			"invalid path passed in request")
	}
	return pathsArr, nil
}

func pathHashFromReq(r *http.Request, allocationID string) (string, error) {
	var (
		pathHash = r.FormValue("path_hash")
		path     = r.FormValue("path")
	)

	if len(pathHash) == 0 {
		if len(path) == 0 {
			return "", common.NewError("invalid_parameters", "Invalid path")
		}
		pathHash = reference.GetReferenceLookup(allocationID, path)
	}

	return pathHash, nil
}
