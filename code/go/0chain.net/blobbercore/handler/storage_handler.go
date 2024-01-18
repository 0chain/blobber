package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"gorm.io/gorm"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/gosdk/constants"
	"go.uber.org/zap"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

const (
	OffsetDateLayout     = "2006-01-02T15:04:05.99999Z07:00"
	DownloadContentFull  = "full"
	DownloadContentThumb = "thumbnail"
	MaxPageLimit         = 100 //100 rows will make up to 100 KB
	DefaultPageLimit     = 20
	DefaultListPageLimit = 50
)

type StorageHandler struct{}

// verifyAllocation try to get allocation from postgres.if it doesn't exists, get it from sharders, and insert it into postgres.
func (fsh *StorageHandler) verifyAllocation(ctx context.Context, allocationID, allocationTx string, readonly bool) (alloc *allocation.Allocation, err error) {

	if allocationTx == "" {
		return nil, common.NewError("verify_allocation",
			"invalid allocation id")
	}

	alloc, err = allocation.FetchAllocationFromEventsDB(ctx, allocationID, allocationTx, readonly)
	if err != nil {
		return nil, common.NewErrorf("verify_allocation",
			"verifying allocation transaction error: %v", err)
	}

	if alloc.Expiration < common.Now() {
		return nil, common.NewError("verify_allocation",
			"use of expired allocation")
	}

	return alloc, nil
}

func (fsh *StorageHandler) convertGormError(err error) error {
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return common.NewError("invalid_path", "path does not exist")
		}
		return common.NewError("db_error", err.Error())
	}
	return err
}

// verifyAuthTicket verifies authTicket and returns authToken and error if any. For any error authToken is nil
func (fsh *StorageHandler) verifyAuthTicket(ctx context.Context, authTokenString string, allocationObj *allocation.Allocation, refRequested *reference.Ref, clientID string, verifyShare bool) (*readmarker.AuthTicket, error) {
	return verifyAuthTicket(ctx, authTokenString, allocationObj, refRequested, clientID, verifyShare)
}

func (fsh *StorageHandler) GetAllocationDetails(ctx context.Context, r *http.Request) (interface{}, error) {
	allocationId := r.FormValue("id")

	allocationObj, err := fsh.verifyAllocation(ctx, allocationId, allocationId, false)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	return allocationObj, nil
}

func (fsh *StorageHandler) GetAllocationUpdateTicket(ctx context.Context, r *http.Request) (interface{}, error) {
	if r.Method != "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use GET instead")
	}
	allocationId := r.FormValue("allocation_id")

	allocationObj, err := fsh.verifyAllocation(ctx, allocationId, allocationId, false)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	// TODO

	return allocationObj, nil
}

func (fsh *StorageHandler) checkIfFileAlreadyExists(ctx context.Context, allocationID, path string) (*reference.Ref, error) {
	return reference.GetLimitedRefFieldsByPath(ctx, allocationID, path, []string{"id", "type"})
}

func (fsh *StorageHandler) GetFileMeta(ctx context.Context, r *http.Request) (interface{}, error) {
	allocationId := ctx.Value(constants.ContextKeyAllocationID).(string)
	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	alloc, err := fsh.verifyAllocation(ctx, allocationId, allocationTx, true)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}
	allocationID := alloc.ID

	clientID := ctx.Value(constants.ContextKeyClient).(string)
	if clientID == "" {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	pathHash, err := pathHashFromReq(r, allocationID)
	if err != nil {
		return nil, err
	}
	fileref, err := reference.GetReferenceByLookupHash(ctx, allocationID, pathHash)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	var (
		isOwner    = clientID == alloc.OwnerID
		isRepairer = clientID == alloc.RepairerID
	)

	if isOwner {
		publicKey := alloc.OwnerPublicKey

		valid, err := verifySignatureFromRequest(allocationTx, r.Header.Get(common.ClientSignatureHeader), publicKey)
		if !valid || err != nil {
			return nil, common.NewError("invalid_signature", "Invalid signature")
		}
	}

	result := fileref.GetListingData(ctx)

	if !isOwner && !isRepairer {
		var authTokenString = r.FormValue("auth_token")

		// check auth token
		if authToken, err := fsh.verifyAuthTicket(ctx, authTokenString, alloc, fileref, clientID, true); authToken == nil {
			return nil, common.NewErrorf("file_meta", "cannot verify auth ticket: %v", err)
		}

		delete(result, "path")
	}

	return result, nil
}

func (fsh *StorageHandler) GetFilesMetaByName(ctx context.Context, r *http.Request, name string) (result []map[string]interface{}, err error) {
	allocationId := ctx.Value(constants.ContextKeyAllocationID).(string)
	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	alloc, err := fsh.verifyAllocation(ctx, allocationId, allocationTx, true)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}
	allocationID := alloc.ID

	clientID := ctx.Value(constants.ContextKeyClient).(string)
	if clientID == "" {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	var (
		isOwner    = clientID == alloc.OwnerID
		isRepairer = clientID == alloc.RepairerID
	)

	if isOwner {
		publicKey := alloc.OwnerPublicKey

		valid, err := verifySignatureFromRequest(allocationTx, r.Header.Get(common.ClientSignatureHeader), publicKey)
		if !valid || err != nil {
			return nil, common.NewError("invalid_signature", "Invalid signature")
		}
	}

	filerefs, err := reference.GetReferencesByName(ctx, allocationID, name)
	if err != nil {
		Logger.Info("No files in current allocation matched the search keyword", zap.Error(err))
		return result, nil
	}

	for _, fileref := range filerefs {
		converted := fileref.GetListingData(ctx)
		result = append(result, converted)
	}

	if !isOwner && !isRepairer {
		var authTokenString = r.FormValue("auth_token")

		// check auth token
		for i, fileref := range filerefs {
			if authToken, err := fsh.verifyAuthTicket(ctx, authTokenString, alloc, fileref, clientID, true); authToken == nil {
				return nil, common.NewErrorf("file_meta", "cannot verify auth ticket: %v", err)
			}

			delete(result[i], "path")
		}
	}

	return result, nil
}

func (fsh *StorageHandler) GetFileStats(ctx context.Context, r *http.Request) (interface{}, error) {
	allocationId := ctx.Value(constants.ContextKeyAllocationID).(string)
	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationId, allocationTx, true)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}
	allocationID := allocationObj.ID

	clientSign, _ := ctx.Value(constants.ContextKeyClientSignatureHeaderKey).(string)
	valid, err := verifySignatureFromRequest(allocationTx, clientSign, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	clientID := ctx.Value(constants.ContextKeyClient).(string)
	if clientID == "" || allocationObj.OwnerID != clientID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	_ = ctx.Value(constants.ContextKeyClientKey).(string)

	pathHash, err := pathHashFromReq(r, allocationID)
	if err != nil {
		return nil, err
	}
	fileref, err := reference.GetReferenceByLookupHash(ctx, allocationID, pathHash)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	if fileref.Type != reference.FILE {
		return nil, common.NewError("invalid_parameters", "Path is not a file.")
	}

	result := fileref.GetListingData(ctx)
	fileStats, err := reference.GetFileStats(ctx, fileref)
	if err != nil {
		return nil, common.NewError("bad_db_operation", "Error retrieving file stats. "+err.Error())
	}
	wm, _ := writemarker.GetWriteMarkerEntity(ctx, fileref.AllocationRoot)
	if wm != nil && fileStats != nil {
		fileStats.WriteMarkerRedeemTxn = wm.CloseTxnID
		fileStats.OnChain = wm.OnChain()
	}
	statsMap := make(map[string]interface{})
	statsBytes, err := json.Marshal(fileStats)
	if err != nil {
		return nil, common.NewError("json_marshal_error", "Error marshaling file stats to JSON. "+err.Error())
	}
	if err := json.Unmarshal(statsBytes, &statsMap); err != nil {
		return nil, common.NewError("json_unmarshal_error", "Error unmarshaling stats bytes to map. "+err.Error())
	}
	for k, v := range statsMap {
		result[k] = v
	}
	return result, nil
}

func (fsh *StorageHandler) ListEntities(ctx context.Context, r *http.Request) (*blobberhttp.ListResult, error) {
	clientID := ctx.Value(constants.ContextKeyClient).(string)
	allocationId := ctx.Value(constants.ContextKeyAllocationID).(string)
	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationId, allocationTx, false)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}
	allocationID := allocationObj.ID

	if clientID == "" {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	pathHash, path, err := getPathHash(r, allocationID)
	if err != nil {
		return nil, err
	}
	_, ok := common.GetField(r, "list")
	escapedPathHash := sanitizeString(pathHash)

	Logger.Info("Path Hash for list dir :" + escapedPathHash)
	fileref, err := reference.GetLimitedRefFieldsByLookupHash(ctx, allocationID, pathHash, []string{"id", "path", "lookup_hash", "type", "name", "file_meta_hash", "parent_path"})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// `/` always is valid even it doesn't exists in db. so ignore RecordNotFound error
			if path != "/" {
				return nil, common.NewError("invalid_parameters", "Invalid path "+err.Error())
			}
		} else {
			return nil, common.NewError("bad_db_operation", err.Error())
		}
	}

	authTokenString, _ := common.GetField(r, "auth_token")
	if clientID != allocationObj.OwnerID || len(authTokenString) > 0 {
		authToken, err := fsh.verifyAuthTicket(ctx, authTokenString, allocationObj, fileref, clientID, true)
		if err != nil {
			return nil, err
		}
		if authToken == nil {
			return nil, common.NewError("auth_ticket_verification_failed", "Could not verify the auth ticket.")
		}
	}

	if !ok {
		var listResult blobberhttp.ListResult
		listResult.AllocationRoot = allocationObj.AllocationRoot
		if fileref == nil {
			fileref = &reference.Ref{Type: reference.DIRECTORY, Path: path, AllocationID: allocationID}
		}
		if fileref.Type == reference.FILE {
			parent, err := reference.GetReference(ctx, allocationID, fileref.ParentPath)
			if err != nil {
				return nil, common.NewError("invalid_parameters", "Invalid path. Parent dir of file not found. "+err.Error())
			}
			fileref = parent
		}
		listResult.Meta = fileref.GetListingData(ctx)
		if clientID != allocationObj.OwnerID {
			delete(listResult.Meta, "path")
		}
		listResult.Entities = make([]map[string]interface{}, 0)
		return &listResult, nil
	}

	// when '/' is not available in database we ignore 'record not found' error. which results into nil fileRef
	// to handle that condition use filePath '/' while file ref is nil and path  is '/'
	filePath := path
	if fileref != nil {
		filePath = fileref.Path
	} else if path != "/" {
		return nil, common.NewError("invalid_parameters", "Invalid path: ref not found ")
	}

	var offset, pageLimit int

	limitStr := r.FormValue("limit")
	if limitStr == "" {
		pageLimit, err = strconv.Atoi(limitStr)
		if err != nil {
			return nil, common.NewError("invalid_parameters", "Invalid limit value "+err.Error())
		}

		if pageLimit > DefaultListPageLimit {
			pageLimit = DefaultListPageLimit
		} else if pageLimit < -1 {
			pageLimit = -1
		}
	} else {
		pageLimit = DefaultListPageLimit

	}
	offsetStr := r.FormValue("offset")
	if offsetStr == "" {
		offset, err = strconv.Atoi(offsetStr)
		if err != nil {
			return nil, common.NewError("invalid_parameters", "Invalid offset value "+err.Error())
		}
	}

	// If the reference is a file, build result with the file and parent dir.
	var dirref *reference.Ref
	if fileref != nil && fileref.Type == reference.FILE {
		r, err := reference.GetReference(ctx, allocationID, filePath)
		if err != nil {
			return nil, common.NewError("invalid_parameters", "Invalid path. "+err.Error())
		}

		parent, err := reference.GetReference(ctx, allocationID, r.ParentPath)
		if err != nil {
			return nil, common.NewError("invalid_parameters", "Invalid path. Parent dir of file not found. "+err.Error())
		}

		parent.Children = append(parent.Children, r)

		dirref = parent
	} else {
		r, err := reference.GetRefWithChildren(ctx, allocationID, filePath, offset, pageLimit)
		if err != nil {
			return nil, common.NewError("invalid_parameters", "Invalid path. "+err.Error())
		}

		dirref = r
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

		if child.Type == reference.DIRECTORY || clientID != allocationObj.OwnerID {
			continue
		}
	}

	return &result, nil
}

func (fsh *StorageHandler) GetLatestWriteMarker(ctx context.Context, r *http.Request) (*blobberhttp.LatestWriteMarkerResult, error) {
	clientID := ctx.Value(constants.ContextKeyClient).(string)
	if clientID == "" {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	allocationId := ctx.Value(constants.ContextKeyAllocationID).(string)
	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationId, allocationTx, false)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	clientSign, _ := ctx.Value(constants.ContextKeyClientSignatureHeaderKey).(string)
	publicKey := allocationObj.OwnerPublicKey

	valid, err := verifySignatureFromRequest(allocationTx, clientSign, publicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "could not verify the allocation owner")
	}

	var latestWM *writemarker.WriteMarkerEntity
	var prevWM *writemarker.WriteMarkerEntity
	if allocationObj.AllocationRoot == "" {
		latestWM = nil
	} else {
		latestWM, err = writemarker.GetWriteMarkerEntity(ctx, allocationObj.AllocationRoot)
		if err != nil {
			Logger.Error("[latest_write_marker]", zap.String("allocation_root", allocationObj.AllocationRoot), zap.String("allocation_id", allocationObj.ID))
			return nil, common.NewError("latest_write_marker_read_error", "Error reading the latest write marker for allocation. "+err.Error())
		}
		if latestWM == nil {
			Logger.Info("[latest_write_marker]", zap.String("allocation_root", allocationObj.AllocationRoot), zap.String("allocation_id", allocationObj.ID))
			return nil, common.NewError("latest_write_marker_read_error", "Latest write marker not found for allocation.")
		}
		if latestWM.WM.PreviousAllocationRoot != "" {
			prevWM, err = writemarker.GetWriteMarkerEntity(ctx, latestWM.WM.PreviousAllocationRoot)
			if err != nil {
				return nil, common.NewError("latest_write_marker_read_error", "Error reading the previous write marker for allocation."+err.Error())
			}
		}
	}

	var result blobberhttp.LatestWriteMarkerResult
	if latestWM != nil {
		result.LatestWM = &latestWM.WM
	}
	if prevWM != nil {
		result.PrevWM = &prevWM.WM
	}

	return &result, nil
}

func (fsh *StorageHandler) GetReferencePath(ctx context.Context, r *http.Request) (*blobberhttp.ReferencePathResult, error) {
	resCh := make(chan *blobberhttp.ReferencePathResult)
	errCh := make(chan error)
	go fsh.getReferencePath(ctx, r, resCh, errCh)

	for {
		select {
		case <-ctx.Done():
			return nil, common.NewError("timeout", "timeout reached")
		case result := <-resCh:
			return result, nil
		case err := <-errCh:
			return nil, err
		}
	}
}

func (fsh *StorageHandler) getReferencePath(ctx context.Context, r *http.Request, resCh chan<- *blobberhttp.ReferencePathResult, errCh chan<- error) {
	allocationId := ctx.Value(constants.ContextKeyAllocationID).(string)
	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationId, allocationTx, false)
	if err != nil {
		errCh <- common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
		return
	}
	allocationID := allocationObj.ID

	paths, err := pathsFromReq(r)
	if err != nil {
		errCh <- err
		return
	}

	clientSign, _ := ctx.Value(constants.ContextKeyClientSignatureHeaderKey).(string)

	clientID := ctx.Value(constants.ContextKeyClient).(string)
	if clientID == "" {
		errCh <- common.NewError("invalid_operation", "Please pass clientID in the header")
		return
	}

	publicKey := allocationObj.OwnerPublicKey

	valid, err := verifySignatureFromRequest(allocationTx, clientSign, publicKey)
	if !valid || err != nil {
		errCh <- common.NewError("invalid_signature", "could not verify the allocation owner or collaborator")
		return
	}
	rootRef, err := reference.GetReferencePathFromPaths(ctx, allocationID, paths, []string{})
	if err != nil {
		errCh <- err
		return
	}

	refPath := &reference.ReferencePath{Ref: rootRef}

	refsToProcess := []*reference.ReferencePath{refPath}

	//convert Ref tree to ReferencePath tree
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
	if allocationObj.AllocationRoot == "" {
		latestWM = nil
	} else {
		latestWM, err = writemarker.GetWriteMarkerEntity(ctx, rootRef.Hash)
		if err != nil {
			errCh <- common.NewError("latest_write_marker_read_error", "Error reading the latest write marker for allocation."+err.Error())
			return
		}
	}

	var refPathResult blobberhttp.ReferencePathResult
	refPathResult.ReferencePath = refPath
	if latestWM != nil {
		refPathResult.LatestWM = &latestWM.WM
	}

	resCh <- &refPathResult
}

func (fsh *StorageHandler) GetObjectTree(ctx context.Context, r *http.Request) (*blobberhttp.ReferencePathResult, error) {

	allocationId := ctx.Value(constants.ContextKeyAllocationID).(string)
	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationId, allocationTx, false)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}
	allocationID := allocationObj.ID

	clientSign, _ := ctx.Value(constants.ContextKeyClientSignatureHeaderKey).(string)
	valid, err := verifySignatureFromRequest(allocationTx, clientSign, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	clientID := ctx.Value(constants.ContextKeyClient).(string)
	if clientID == "" || allocationObj.OwnerID != clientID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}
	path := r.FormValue("path")
	if path == "" {
		return nil, common.NewError("invalid_parameters", "Invalid path")
	}

	rootRef, err := reference.GetObjectTree(ctx, allocationID, path)
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
	if allocationObj.AllocationRoot == "" {
		latestWM = nil
	} else {
		latestWM, err = writemarker.GetWriteMarkerEntity(ctx, allocationObj.AllocationRoot)
		if err != nil {
			return nil, common.NewError("latest_write_marker_read_error", "Error reading the latest write marker for allocation."+err.Error())
		}
	}
	var refPathResult blobberhttp.ReferencePathResult
	refPathResult.ReferencePath = refPath
	if latestWM != nil {
		refPathResult.LatestWM = &latestWM.WM
	}
	return &refPathResult, nil
}

func (fsh *StorageHandler) GetRecentlyAddedRefs(ctx context.Context, r *http.Request) (*blobberhttp.RecentRefResult, error) {
	allocationId := ctx.Value(constants.ContextKeyAllocationID).(string)
	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationId, allocationTx, false)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	clientID := ctx.Value(constants.ContextKeyClient).(string)
	if clientID == "" {
		return nil, common.NewError("invalid_operation", "Client id is required")
	}

	clientSign := ctx.Value(constants.ContextKeyClientSignatureHeaderKey).(string)

	valid, err := verifySignatureFromRequest(allocationTx, clientSign, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature or invalid access")
	}

	allocationID := allocationObj.ID

	var offset, pageLimit int
	offsetStr := r.FormValue("offset")

	if offsetStr != "" {
		offset, err = strconv.Atoi(offsetStr)
		if err != nil {
			return nil, common.NewError("invalid_parameters", "Invalid offset value "+err.Error())
		}
	}

	pageLimitStr := r.FormValue("limit")
	if pageLimitStr != "" {
		pageLimit, err = strconv.Atoi(pageLimitStr)
		if err != nil {
			return nil, common.NewError("invalid_parameters", "Invalid page limit value. Got Error "+err.Error())
		}
		if pageLimit < 0 {
			return nil, common.NewError("invalid_parameters", "Zero/Negative page limit value is not allowed")
		}

		if pageLimit > MaxPageLimit {
			pageLimit = MaxPageLimit
		}

	} else {
		pageLimit = DefaultPageLimit
	}

	var fromDate int
	fromDateStr := r.FormValue("from-date")
	if fromDateStr != "" {
		fromDate, err = strconv.Atoi(fromDateStr)
		if err != nil {
			return nil, common.NewError("invalid_parameters", "Invalid from date value. Got error "+err.Error())
		}
		if fromDate < 0 {
			return nil, common.NewError("invalid_parameters", "Negative from date value is not allowed")
		}
	}

	refs, offset, err := reference.GetRecentlyCreatedRefs(ctx, allocationID, pageLimit, offset, fromDate)
	if err != nil {
		return nil, err
	}

	return &blobberhttp.RecentRefResult{
		Refs:   refs,
		Offset: offset,
	}, nil
}

// Retrieves file refs. One can use three types to refer to regular, updated and deleted. Regular type gives all undeleted rows.
// Updated gives rows that is updated compared to the date given. And deleted gives deleted refs compared to the date given.
// Updated date time format should be as declared in above constant; OffsetDateLayout
func (fsh *StorageHandler) GetRefs(ctx context.Context, r *http.Request) (*blobberhttp.RefResult, error) {
	allocationId := ctx.Value(constants.ContextKeyAllocationID).(string)
	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationId, allocationTx, false)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	clientID := ctx.Value(constants.ContextKeyClient).(string)
	if clientID == "" {
		return nil, common.NewError("invalid_operation", "Client id is required")
	}

	publicKey, _ := ctx.Value(constants.ContextKeyClientKey).(string)
	if publicKey == "" {
		if clientID == allocationObj.OwnerID {
			publicKey = allocationObj.OwnerPublicKey
		} else {
			return nil, common.NewError("empty_public_key", "public key is required")
		}
	}

	clientSign, _ := ctx.Value(constants.ContextKeyClientSignatureHeaderKey).(string)

	valid, err := verifySignatureFromRequest(allocationTx, clientSign, publicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	allocationID := allocationObj.ID

	path := r.FormValue("path")
	authTokenStr := r.FormValue("auth_token")
	pathHash := r.FormValue("path_hash")

	if path == "" && authTokenStr == "" && pathHash == "" {
		return nil, common.NewError("invalid_parameters", "empty path and authtoken")
	}

	var pathRef *reference.Ref
	switch {
	case path != "":
		pathHash = reference.GetReferenceLookup(allocationID, path)
		fallthrough
	case pathHash != "":
		pathRef, err = reference.GetReferenceByLookupHash(ctx, allocationID, pathHash)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, common.NewError("invalid_path", "")
			}
			return nil, err
		}

		if clientID == allocationObj.OwnerID || clientID == allocationObj.RepairerID {
			break
		}
		if authTokenStr == "" {
			return nil, common.NewError("unauthorized_request", "client is not authorized for the requested resource")
		}
		fallthrough
	case authTokenStr != "":
		authToken := &readmarker.AuthTicket{}
		if err := json.Unmarshal([]byte(authTokenStr), authToken); err != nil {
			return nil, common.NewError("json_unmarshall_error", fmt.Sprintf("error parsing authticket: %v", authTokenStr))
		}

		shareInfo, err := reference.GetShareInfo(ctx, authToken.ClientID, authToken.FilePathHash)
		if err != nil {
			return nil, fsh.convertGormError(err)
		}
		if shareInfo.Revoked {
			return nil, common.NewError("share_revoked", "client no longer has permission to requested resource")
		}

		if err := authToken.Verify(allocationObj, clientID); err != nil {
			return nil, err
		}

		if pathRef == nil {
			pathRef, err = reference.GetReferenceByLookupHash(ctx, allocationID, authToken.FilePathHash)
			if err != nil {
				return nil, fsh.convertGormError(err)
			}
		} else if pathHash != authToken.FilePathHash {
			authTokenRef, err := reference.GetReferenceByLookupHash(ctx, allocationID, authToken.FilePathHash)
			if err != nil {
				return nil, fsh.convertGormError(err)
			}
			matched, _ := regexp.MatchString(fmt.Sprintf("^%v", authTokenRef.Path), pathRef.Path)
			if !matched {
				return nil, common.NewError("invalid_authticket", "auth ticket is not valid for requested resource")
			}
		}
	default:
		return nil, common.NewError("incomplete_request", "path, pathHash or authTicket is required")
	}

	path = pathRef.Path
	pageLimitStr := r.FormValue("pageLimit")
	var pageLimit int
	if pageLimitStr == "" {
		pageLimit = DefaultPageLimit
	} else {
		o, err := strconv.Atoi(pageLimitStr)
		if err != nil {
			return nil, common.NewError("invalid_parameters", "Invalid page limit value type")
		}
		if o <= 0 {
			return nil, common.NewError("invalid_parameters", "Zero/Negative page limit value is not allowed")
		} else if o > MaxPageLimit {
			pageLimit = MaxPageLimit
		} else {
			pageLimit = o
		}
	}

	offsetPath := r.FormValue("offsetPath")
	offsetDate := r.FormValue("offsetDate")
	updatedDate := r.FormValue("updatedDate")
	err = checkValidDate(offsetDate, OffsetDateLayout)
	if err != nil {
		return nil, err
	}
	err = checkValidDate(updatedDate, OffsetDateLayout)
	if err != nil {
		return nil, err
	}
	fileType := r.FormValue("fileType")
	levelStr := r.FormValue("level")
	var level int
	if levelStr != "" {
		level, err = strconv.Atoi(levelStr)
		if err != nil {
			return nil, common.NewError("invalid_parameters", err.Error())
		}
		if level < 0 {
			return nil, common.NewError("invalid_parameters", "Negative level value is not allowed")
		}
	}

	refType := r.FormValue("refType")

	var refs *[]reference.PaginatedRef
	var totalPages int
	var newOffsetPath string
	var newOffsetDate common.Timestamp

	switch {
	case refType == "regular":
		refs, totalPages, newOffsetPath, err = reference.GetRefs(
			ctx, allocationID, path, offsetPath, fileType, level, pageLimit,
		)

	case refType == "updated":
		refs, totalPages, newOffsetPath, newOffsetDate, err = reference.GetUpdatedRefs(
			ctx, allocationID, path, offsetPath, fileType,
			updatedDate, offsetDate, level, pageLimit, OffsetDateLayout,
		)

	default:
		return nil, common.NewError("invalid_parameters", "refType param should have value regular/updated/deleted")
	}

	if err != nil {
		return nil, err
	}
	var latestWM *writemarker.WriteMarkerEntity
	if allocationObj.AllocationRoot == "" {
		latestWM = nil
	} else {
		latestWM, err = writemarker.GetWriteMarkerEntity(ctx, allocationObj.AllocationRoot)
		if err != nil {
			return nil, common.NewError("latest_write_marker_read_error", "Error reading the latest write marker for allocation."+err.Error())
		}
	}

	var refResult blobberhttp.RefResult
	refResult.Refs = refs
	refResult.TotalPages = totalPages
	refResult.OffsetPath = newOffsetPath
	refResult.OffsetDate = newOffsetDate
	if latestWM != nil {
		refResult.LatestWM = &latestWM.WM
	}
	// Refs will be returned as it is and object tree will be build in client side
	return &refResult, nil
}

// verifySignatureFromRequest verifies signature passed as common.ClientSignatureHeader header.
func verifySignatureFromRequest(alloc, sign, pbK string) (bool, error) {
	sign = encryption.MiraclToHerumiSig(sign)

	if len(sign) < 64 {
		return false, nil
	}

	hash := encryption.Hash(alloc)
	return encryption.Verify(pbK, sign, hash)
}

// pathsFromReq retrieves paths value from request which can be represented as single "path" value or "paths" values,
// marshaled to json.
func pathsFromReq(r *http.Request) ([]string, error) {
	var (
		pathsStr = r.FormValue("paths")
		path     = r.FormValue("path")
		paths    = make([]string, 0)
	)

	if pathsStr == "" {
		if path == "" {
			return nil, common.NewError("invalid_parameters", "Invalid path")
		}

		return append(paths, path), nil
	}

	if err := json.Unmarshal([]byte(pathsStr), &paths); err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid path array json")
	}

	return paths, nil
}

func pathHashFromReq(r *http.Request, allocationID string) (pathHash string, err error) {
	pathHash, _, err = getPathHash(r, allocationID)
	return
}

func getPathHash(r *http.Request, allocationID string) (pathHash, path string, err error) {
	pathHash, _ = common.GetField(r, "path_hash")
	path, _ = common.GetField(r, "path")

	if pathHash == "" && path == "" {
		pathHash = r.Header.Get("path_hash")
		path = r.Header.Get("path")
	}

	if pathHash == "" {
		if path == "" {
			return "", "", common.NewError("invalid_parameters", "Invalid path")
		}
		pathHash = reference.GetReferenceLookup(allocationID, path)
	}

	return pathHash, path, nil
}
