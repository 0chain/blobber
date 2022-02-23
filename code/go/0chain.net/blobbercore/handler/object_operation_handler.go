package handler

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"

	"net/http"
	"path/filepath"
	"strconv"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/lock"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/gosdk/constants"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

const (
	// EncryptionHeaderSize encryption header size in chunk: PRE.MessageChecksum(128)"+PRE.OverallChecksum(128)
	EncryptionHeaderSize = 128 + 128
	// ReEncryptionHeaderSize re-encryption header size in chunk
	ReEncryptionHeaderSize = 256
)

func readPreRedeem(ctx context.Context, alloc *allocation.Allocation, numBlocks, pendNumBlocks int64, payerID string) (err error) {
	if numBlocks == 0 {
		return
	}

	// check out read pool tokens if read_price > 0
	var (
		db        = datastore.GetStore().GetTransaction(ctx)
		blobberID = node.Self.ID
		until     = common.Now() +
			common.Timestamp(config.Configuration.ReadLockTimeout)

		want = alloc.WantRead(blobberID, numBlocks)

		rps []*allocation.ReadPool
	)

	if want == 0 {
		return // skip if read price is zero
	}

	rps, err = allocation.ReadPools(db, payerID, alloc.ID,
		blobberID, until)
	if err != nil {
		return common.NewErrorf("read_pre_redeem",
			"can't get read pools from DB: %v", err)
	}

	var have = alloc.HaveRead(rps, blobberID, pendNumBlocks)

	if have < want {
		rps, err = allocation.RequestReadPools(payerID,
			alloc.ID)
		if err != nil {
			return common.NewErrorf("read_pre_redeem",
				"can't request read pools from sharders: %v", err)
		}

		err = allocation.SetReadPools(db, payerID,
			alloc.ID, blobberID, rps)
		if err != nil {
			return common.NewErrorf("read_pre_redeem",
				"can't save requested read pools: %v", err)
		}

		rps, err = allocation.ReadPools(db, payerID, alloc.ID, blobberID,
			until)
		if err != nil {
			return common.NewErrorf("read_pre_redeem",
				"can't get read pools from DB: %v", err)
		}

		have = alloc.HaveRead(rps, blobberID, pendNumBlocks)
	}

	if have < want {
		return common.NewError("read_pre_redeem", "not enough "+
			"tokens in client's read pools associated with the"+
			" allocation->blobber")
	}

	return
}

func writePreRedeem(ctx context.Context, alloc *allocation.Allocation, writeMarker *writemarker.WriteMarker, payerID string) (err error) {
	// check out read pool tokens if read_price > 0
	var (
		db        = datastore.GetStore().GetTransaction(ctx)
		blobberID = node.Self.ID
		until     = common.Now() +
			common.Timestamp(config.Configuration.WriteLockTimeout)

		want = alloc.WantWrite(blobberID, writeMarker.Size,
			writeMarker.Timestamp)

		pend *allocation.Pending
		wps  []*allocation.WritePool
	)

	if writeMarker.Size <= 0 || want <= 0 {
		return // skip if write price is zero or it's about deleting
	}

	pend, err = allocation.GetPending(db, payerID,
		alloc.ID, blobberID)
	if err != nil {
		return common.NewErrorf("write_pre_redeem",
			"can't get pending payments: %v", err)
	}

	wps, err = pend.WritePools(db, blobberID, until)
	if err != nil {
		return common.NewErrorf("write_pre_redeem",
			"can't get read pools from DB: %v", err)
	}

	var have = pend.HaveWrite(wps, alloc, writeMarker.Timestamp)
	if have < want {
		wps, err = allocation.RequestWritePools(payerID,
			alloc.ID)
		if err != nil {
			return common.NewErrorf("write_pre_redeem",
				"can't request write pools from sharders: %v", err)
		}
		err = allocation.SetWritePools(db, payerID,
			alloc.ID, blobberID, wps)
		if err != nil {
			return common.NewErrorf("write_pre_redeem",
				"can't save requested write pools: %v", err)
		}
		wps, err = pend.WritePools(db, blobberID, until)
		if err != nil {
			return common.NewErrorf("write_pre_redeem",
				"can't get write pools from DB: %v", err)
		}
		have = pend.HaveWrite(wps, alloc, writeMarker.Timestamp)
	}

	if have < want {
		return common.NewErrorf("write_pre_redeem", "not enough "+
			"tokens in write pools (client -> allocation ->  blobber)"+
			"(%s -> %s -> %s), have %d, want %d", payerID,
			alloc.ID, writeMarker.BlobberID, have, want)
	}

	// update pending writes: add size to redeem to (not tokens)
	pend.AddPendingWrite(writeMarker.Size)
	if err = pend.Save(db); err != nil {
		return common.NewErrorf("write_pre_redeem",
			"can't save pending writes in DB: %v", err)
	}

	return
}

func (fsh *StorageHandler) DownloadFile(ctx context.Context, r *http.Request) (resp interface{}, err error) {
	// get client and allocation ids
	var (
		clientID     = ctx.Value(constants.ContextKeyClient).(string)
		allocationTx = ctx.Value(constants.ContextKeyAllocation).(string)
		alloc        *allocation.Allocation
	)

	if clientID == "" {
		return nil, common.NewError("download_file", "invalid client")
	}

	alloc, err = fsh.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewErrorf("download_file", "invalid allocation id passed: %v", err)
	}

	pathHash, _, err := getPathHash(r, alloc.ID)
	if err != nil {
		return nil, common.NewError("download_file", "invalid path")
	}

	var blockNumStr = r.Header.Get("block_num")
	if blockNumStr == "" {
		return nil, common.NewError("download_file", "no block number")
	}

	var blockNum int64
	blockNum, err = strconv.ParseInt(blockNumStr, 10, 64)
	if err != nil || blockNum < 1 {
		return nil, common.NewError("download_file", "invalid block number")
	}

	numBlocks := int64(1)
	numBlocksStr := r.Header.Get("num_blocks")

	if numBlocksStr != "" {
		numBlocks, err = strconv.ParseInt(numBlocksStr, 10, 64)
		if err != nil || numBlocks <= 0 {
			return nil, common.NewError("download_file", "invalid number of blocks")
		}
	}

	readMarkerString := r.Header.Get("read_marker")
	readMarker := new(readmarker.ReadMarker)

	if err := json.Unmarshal([]byte(readMarkerString), readMarker); err != nil {
		return nil, common.NewErrorf("download_file", "invalid parameters, "+
			"error parsing the readmarker for download: %v", err)
	}

	rmObj := new(readmarker.ReadMarkerEntity)
	rmObj.LatestRM = readMarker

	if err = rmObj.VerifyMarker(ctx, alloc); err != nil {
		return nil, common.NewErrorf("download_file", "invalid read marker, "+"failed to verify the read marker: %v", err)
	}

	// get file reference
	fileref, err := reference.GetReferenceFromLookupHash(ctx, alloc.ID, pathHash)
	if err != nil {
		return nil, common.NewErrorf("download_file", "invalid file path: %v", err)
	}

	if fileref.Type != reference.FILE {
		return nil, common.NewErrorf("download_file", "path is not a file: %v", err)
	}

	// set payer: default
	payerID := alloc.OwnerID

	// set payer: check for explicit allocation payer value
	if alloc.PayerID != "" {
		payerID = alloc.PayerID
	}

	isOwner := clientID == alloc.OwnerID
	isRepairer := clientID == alloc.RepairerID
	isCollaborator := reference.IsACollaborator(ctx, fileref.ID, clientID)

	var authToken *readmarker.AuthTicket
	var shareInfo *reference.ShareInfo

	if !(isOwner || isCollaborator) {
		authTokenString := r.Header.Get("auth_token")
		if authTokenString == "" {
			return nil, common.NewError("invalid_client", "authticket is required")
		}

		if authToken, err = fsh.verifyAuthTicket(ctx, authTokenString, alloc, fileref, clientID); authToken == nil {
			return nil, common.NewErrorf("download_file", "cannot verify auth ticket: %v", err)
		}

		shareInfo, err = reference.GetShareInfo(ctx, authToken.ClientID, authToken.FilePathHash)
		if err != nil || shareInfo == nil {
			return nil, errors.New("client does not have permission to download the file. share does not exist")
		}

		if shareInfo.Revoked {
			return nil, errors.New("client does not have permission to download the file. share revoked")
		}

		// set payer: check for command line payer flag (--rx_pay)
		if r.Header.Get("rx_pay") == "true" {
			payerID = clientID
		}

		readMarker.AuthTicket = datatypes.JSON(authTokenString)

		// check for file payer flag
		if fileAttrs, err := fileref.GetAttributes(); err != nil {
			return nil, common.NewErrorf("download_file", "error getting file attributes: %v", err)
		} else if fileAttrs.WhoPaysForReads == common.WhoPays3rdParty && !(isCollaborator || isRepairer) {
			payerID = clientID
		}
	}
	// create read marker
	var (
		rme           *readmarker.ReadMarkerEntity
		latestRM      *readmarker.ReadMarker
		pendNumBlocks int64
	)

	rme, err = readmarker.GetLatestReadMarkerEntity(ctx, clientID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.NewErrorf("download_file", "couldn't get read marker from DB: %v", err)
	}

	if rme != nil {
		latestRM = rme.LatestRM
		if pendNumBlocks, err = rme.PendNumBlocks(); err != nil {
			return nil, common.NewErrorf("download_file", "couldn't get number of blocks pending redeeming: %v", err)
		}
	}

	if latestRM != nil && latestRM.ReadCounter+(numBlocks) != readMarker.ReadCounter {
		return &blobberhttp.DownloadResponse{
			Success:      false,
			LatestRM:     latestRM,
			Path:         fileref.Path,
			AllocationID: fileref.AllocationID,
		}, nil
	}

	// check out read pool tokens if read_price > 0
	err = readPreRedeem(ctx, alloc, numBlocks, pendNumBlocks, payerID)
	if err != nil {
		return nil, common.NewErrorf("download_file", "pre-redeeming read marker: %v", err)
	}

	// reading is allowed
	var (
		downloadMode = r.Header.Get("content")
		respData     []byte
	)
	if downloadMode == DownloadContentThumb {
		var fileData = &filestore.FileInputData{}
		fileData.Name = fileref.Name
		fileData.Path = fileref.Path
		fileData.Hash = fileref.ThumbnailHash
		fileData.OnCloud = fileref.OnCloud
		fileData.ChunkSize = fileref.ChunkSize
		respData, err = filestore.GetFileStore().GetFileBlock(alloc.ID, fileData, blockNum, numBlocks)
		if err != nil {
			return nil, common.NewErrorf("download_file", "couldn't get thumbnail block: %v", err)
		}
	} else {
		var fileData = &filestore.FileInputData{}
		fileData.Name = fileref.Name
		fileData.Path = fileref.Path
		fileData.Hash = fileref.ContentHash
		fileData.OnCloud = fileref.OnCloud
		fileData.ChunkSize = fileref.ChunkSize

		respData, err = filestore.GetFileStore().GetFileBlock(alloc.ID, fileData, blockNum, numBlocks)
		if err != nil {
			return nil, common.NewErrorf("download_file", "couldn't get file block: %v", err)
		}
	}

	readMarker.PayerID = payerID
	err = readmarker.SaveLatestReadMarker(ctx, readMarker, latestRM == nil)
	if err != nil {
		Logger.Error(err.Error())
		return nil, common.NewErrorf("download_file", "couldn't save latest read marker")
	}

	var chunkEncoder ChunkEncoder
	if len(fileref.EncryptedKey) > 0 && authToken != nil {
		chunkEncoder = &PREChunkEncoder{
			EncryptedKey:              fileref.EncryptedKey,
			ReEncryptionKey:           shareInfo.ReEncryptionKey,
			ClientEncryptionPublicKey: shareInfo.ClientEncryptionPublicKey,
		}
	} else {
		chunkEncoder = &RawChunkEncoder{}
	}

	chunkData, err := chunkEncoder.Encode(int(fileref.ChunkSize), respData)

	if err != nil {
		return nil, err
	}

	stats.FileBlockDownloaded(ctx, fileref.ID)
	return chunkData, nil
}

func (fsh *StorageHandler) CommitWrite(ctx context.Context, r *http.Request) (*blobberhttp.CommitResult, error) {
	if r.Method == "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used for the upload URL. Use POST instead")
	}

	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	clientID := ctx.Value(constants.ContextKeyClient).(string)
	clientKey := ctx.Value(constants.ContextKeyClientKey).(string)
	clientKeyBytes, _ := hex.DecodeString(clientKey)

	allocationObj, err := fsh.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	if allocationObj.IsImmutable {
		return nil, common.NewError("immutable_allocation", "Cannot write to an immutable allocation")
	}

	allocationID := allocationObj.ID

	connectionID := r.FormValue("connection_id")
	if connectionID == "" {
		return nil, common.NewError("invalid_parameters", "Invalid connection id passed")
	}

	mutex := lock.GetMutex(allocationObj.TableName(), allocationID)
	mutex.Lock()
	defer mutex.Unlock()

	connectionObj, err := allocation.GetAllocationChanges(ctx, connectionID, allocationID, clientID)
	if err != nil {
		return nil, common.NewErrorf("invalid_parameters",
			"Invalid connection id. Connection id was not found: %v", err)
	}
	if len(connectionObj.Changes) == 0 {
		return nil, common.NewError("invalid_parameters",
			"Invalid connection id. Connection does not have any changes.")
	}

	var isCollaborator bool
	for _, change := range connectionObj.Changes {
		if change.Operation != constants.FileOperationUpdate {
			continue
		}

		updateFileChange := new(allocation.UpdateFileChanger)
		if err := updateFileChange.Unmarshal(change.Input); err != nil {
			return nil, err
		}
		fileRef, err := reference.GetReference(ctx, allocationID, updateFileChange.Path)
		if err != nil {
			return nil, err
		}
		isCollaborator = reference.IsACollaborator(ctx, fileRef.ID, clientID)
		break
	}

	if clientID == "" || clientKey == "" {
		return nil, common.NewError("invalid_params", "Please provide clientID and clientKey")
	}

	if (allocationObj.OwnerID != clientID || encryption.Hash(clientKeyBytes) != clientID) && !isCollaborator {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	if err = r.ParseMultipartForm(FormFileParseMaxMemory); err != nil {
		Logger.Info("Error Parsing the request", zap.Any("error", err))
		return nil, common.NewError("request_parse_error", err.Error())
	}

	if allocationObj.BlobberSizeUsed+connectionObj.Size > allocationObj.BlobberSize {
		return nil, common.NewError("max_allocation_size",
			"Max size reached for the allocation with this blobber")
	}

	writeMarkerString := r.FormValue("write_marker")
	writeMarker := writemarker.WriteMarker{}
	err = json.Unmarshal([]byte(writeMarkerString), &writeMarker)
	if err != nil {
		return nil, common.NewErrorf("invalid_parameters",
			"Invalid parameters. Error parsing the writemarker for commit: %v",
			err)
	}

	var result blobberhttp.CommitResult
	var latestWM *writemarker.WriteMarkerEntity
	if allocationObj.AllocationRoot == "" {
		latestWM = nil
	} else {
		latestWM, err = writemarker.GetWriteMarkerEntity(ctx,
			allocationObj.AllocationRoot)
		if err != nil {
			return nil, common.NewErrorf("latest_write_marker_read_error",
				"Error reading the latest write marker for allocation: %v", err)
		}
	}

	writemarkerObj := &writemarker.WriteMarkerEntity{}
	writemarkerObj.WM = writeMarker

	err = writemarkerObj.VerifyMarker(ctx, allocationObj, connectionObj)
	if err != nil {
		result.AllocationRoot = allocationObj.AllocationRoot
		result.ErrorMessage = "Verification of write marker failed: " + err.Error()
		result.Success = false
		if latestWM != nil {
			result.WriteMarker = &latestWM.WM
		}
		return &result, common.NewError("write_marker_verification_failed", result.ErrorMessage)
	}

	var clientIDForWriteRedeem = writeMarker.ClientID
	if isCollaborator {
		clientIDForWriteRedeem = allocationObj.OwnerID
	}

	if err := writePreRedeem(ctx, allocationObj, &writeMarker, clientIDForWriteRedeem); err != nil {
		return nil, err
	}

	err = connectionObj.ApplyChanges(ctx, writeMarker.AllocationRoot)
	if err != nil {
		return nil, err
	}
	rootRef, err := reference.GetReference(ctx, allocationID, "/")
	if err != nil {
		return nil, err
	}

	allocationRoot := encryption.Hash(rootRef.Hash + ":" + strconv.FormatInt(int64(writeMarker.Timestamp), 10))

	if allocationRoot != writeMarker.AllocationRoot {
		result.AllocationRoot = allocationObj.AllocationRoot
		if latestWM != nil {
			result.WriteMarker = &latestWM.WM
		}
		result.Success = false
		result.ErrorMessage = "Allocation root in the write marker does not match the calculated allocation root. Expected hash: " + allocationRoot
		return &result, common.NewError("allocation_root_mismatch", result.ErrorMessage)
	}
	writemarkerObj.ConnectionID = connectionObj.ConnectionID
	writemarkerObj.ClientPublicKey = clientKey
	err = writemarkerObj.Save(ctx)
	if err != nil {
		return nil, common.NewError("write_marker_error", "Error persisting the write marker")
	}

	db := datastore.GetStore().GetTransaction(ctx)
	allocationUpdates := make(map[string]interface{})
	allocationUpdates["blobber_size_used"] = gorm.Expr("blobber_size_used + ?", connectionObj.Size)
	allocationUpdates["used_size"] = gorm.Expr("used_size + ?", connectionObj.Size)
	allocationUpdates["allocation_root"] = allocationRoot
	allocationUpdates["is_redeem_required"] = true

	err = db.Model(allocationObj).Updates(allocationUpdates).Error
	if err != nil {
		return nil, common.NewError("allocation_write_error", "Error persisting the allocation object")
	}
	err = connectionObj.CommitToFileStore(ctx)
	if err != nil {
		return nil, common.NewError("file_store_error", "Error committing to file store. "+err.Error())
	}

	result.Changes = connectionObj.Changes

	connectionObj.DeleteChanges(ctx) //nolint:errcheck // never returns an error anyway

	db.Model(connectionObj).Updates(allocation.AllocationChangeCollector{Status: allocation.CommittedConnection})

	result.AllocationRoot = allocationObj.AllocationRoot
	result.WriteMarker = &writeMarker
	result.Success = true
	result.ErrorMessage = ""

	return &result, nil
}

func (fsh *StorageHandler) RenameObject(ctx context.Context, r *http.Request) (interface{}, error) {
	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	if allocationObj.IsImmutable {
		return nil, common.NewError("immutable_allocation", "Cannot rename data in an immutable allocation")
	}

	allocationID := allocationObj.ID

	clientID := ctx.Value(constants.ContextKeyClient).(string)
	_ = ctx.Value(constants.ContextKeyClientKey).(string)

	valid, err := verifySignatureFromRequest(allocationTx, r.Header.Get(common.ClientSignatureHeader), allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	if clientID == "" {
		return nil, common.NewError("invalid_operation", "Invalid client")
	}

	new_name := r.FormValue("new_name")
	if new_name == "" {
		return nil, common.NewError("invalid_parameters", "Invalid name")
	}

	pathHash, err := pathHashFromReq(r, allocationID)
	if err != nil {
		return nil, err
	}

	if clientID == "" || allocationObj.OwnerID != clientID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	connectionID := r.FormValue("connection_id")
	if connectionID == "" {
		return nil, common.NewError("invalid_parameters", "Invalid connection id passed")
	}

	connectionObj, err := allocation.GetAllocationChanges(ctx, connectionID, allocationID, clientID)
	if err != nil {
		return nil, common.NewError("meta_error", "Error reading metadata for connection")
	}

	mutex := lock.GetMutex(connectionObj.TableName(), connectionID)
	mutex.Lock()
	defer mutex.Unlock()

	objectRef, err := reference.GetReferenceFromLookupHash(ctx, allocationID, pathHash)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	allocationChange := &allocation.AllocationChange{}
	allocationChange.ConnectionID = connectionObj.ConnectionID
	allocationChange.Size = 0
	allocationChange.Operation = constants.FileOperationRename
	dfc := &allocation.RenameFileChange{ConnectionID: connectionObj.ConnectionID,
		AllocationID: connectionObj.AllocationID, Path: objectRef.Path}
	dfc.NewName = new_name
	connectionObj.Size += allocationChange.Size
	connectionObj.AddChange(allocationChange, dfc)

	err = connectionObj.Save(ctx)
	if err != nil {
		Logger.Error("Error in writing the connection meta data", zap.Error(err))
		return nil, common.NewError("connection_write_error", "Error writing the connection meta data")
	}

	result := &blobberhttp.UploadResult{}
	result.Filename = new_name
	result.Hash = objectRef.Hash
	result.MerkleRoot = objectRef.MerkleRoot
	result.Size = objectRef.Size

	return result, nil
}

func (fsh *StorageHandler) UpdateObjectAttributes(ctx context.Context, r *http.Request) (resp interface{}, err error) {
	if r.Method != http.MethodPost {
		return nil, common.NewError("update_object_attributes",
			"Invalid method used. Use POST instead")
	}

	var (
		allocTx  = ctx.Value(constants.ContextKeyAllocation).(string)
		clientID = ctx.Value(constants.ContextKeyClient).(string)

		alloc *allocation.Allocation
	)

	if alloc, err = fsh.verifyAllocation(ctx, allocTx, false); err != nil {
		return nil, common.NewErrorf("update_object_attributes",
			"Invalid allocation ID passed: %v", err)
	}

	valid, err := verifySignatureFromRequest(allocTx, r.Header.Get(common.ClientSignatureHeader), alloc.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	if alloc.IsImmutable {
		return nil, common.NewError("immutable_allocation", "Cannot update data in an immutable allocation")
	}

	// runtime type check
	_ = ctx.Value(constants.ContextKeyClientKey).(string)

	if clientID == "" {
		return nil, common.NewError("update_object_attributes",
			"missing client ID")
	}

	var attributes = r.FormValue("attributes") // new attributes as string
	if attributes == "" {
		return nil, common.NewError("update_object_attributes",
			"missing new attributes, pass at least {} for empty attributes")
	}

	var attrs = new(reference.Attributes)
	if err = json.Unmarshal([]byte(attributes), attrs); err != nil {
		return nil, common.NewErrorf("update_object_attributes",
			"decoding given attributes: %v", err)
	}

	pathHash, err := pathHashFromReq(r, alloc.ID)
	if err != nil {
		return nil, common.NewError("update_object_attributes",
			"missing path and path_hash")
	}

	if alloc.OwnerID != clientID {
		return nil, common.NewError("update_object_attributes",
			"operation needs to be performed by the owner of the allocation")
	}

	var connID = r.FormValue("connection_id")
	if connID == "" {
		return nil, common.NewErrorf("update_object_attributes",
			"invalid connection id passed: %s", connID)
	}

	var conn *allocation.AllocationChangeCollector
	conn, err = allocation.GetAllocationChanges(ctx, connID, alloc.ID, clientID)
	if err != nil {
		return nil, common.NewErrorf("update_object_attributes",
			"reading metadata for connection: %v", err)
	}

	var mutex = lock.GetMutex(conn.TableName(), connID)

	mutex.Lock()
	defer mutex.Unlock()

	var ref *reference.Ref
	ref, err = reference.GetReferenceFromLookupHash(ctx, alloc.ID, pathHash)
	if err != nil {
		return nil, common.NewErrorf("update_object_attributes",
			"invalid file path: %v", err)
	}

	var change = new(allocation.AllocationChange)
	change.ConnectionID = conn.ConnectionID
	change.Operation = constants.FileOperationUpdateAttrs

	var uafc = &allocation.AttributesChange{
		ConnectionID: conn.ConnectionID,
		AllocationID: conn.AllocationID,
		Path:         ref.Path,
		Attributes:   attrs,
	}

	conn.AddChange(change, uafc)

	err = conn.Save(ctx)
	if err != nil {
		Logger.Error("update_object_attributes: "+
			"error in writing the connection meta data", zap.Error(err))
		return nil, common.NewError("update_object_attributes",
			"error writing the connection meta data")
	}

	// return new attributes as result
	return attrs, nil
}

func (fsh *StorageHandler) CopyObject(ctx context.Context, r *http.Request) (interface{}, error) {
	if r.Method == "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used. Use POST instead")
	}
	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	valid, err := verifySignatureFromRequest(allocationTx, r.Header.Get(common.ClientSignatureHeader), allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	if allocationObj.IsImmutable {
		return nil, common.NewError("immutable_allocation", "Cannot copy data in an immutable allocation")
	}

	clientID := ctx.Value(constants.ContextKeyClient).(string)
	_ = ctx.Value(constants.ContextKeyClientKey).(string)

	allocationID := allocationObj.ID

	if clientID == "" {
		return nil, common.NewError("invalid_operation", "Invalid client")
	}

	destPath := r.FormValue("dest")
	if destPath == "" {
		return nil, common.NewError("invalid_parameters", "Invalid destination for operation")
	}

	pathHash, err := pathHashFromReq(r, allocationID)
	if err != nil {
		return nil, err
	}

	if clientID == "" || allocationObj.OwnerID != clientID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	connectionID := r.FormValue("connection_id")
	if connectionID == "" {
		return nil, common.NewError("invalid_parameters", "Invalid connection id passed")
	}

	connectionObj, err := allocation.GetAllocationChanges(ctx, connectionID, allocationID, clientID)
	if err != nil {
		return nil, common.NewError("meta_error", "Error reading metadata for connection")
	}

	mutex := lock.GetMutex(connectionObj.TableName(), connectionID)
	mutex.Lock()
	defer mutex.Unlock()

	objectRef, err := reference.GetReferenceFromLookupHash(ctx, allocationID, pathHash)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}
	newPath := filepath.Join(destPath, objectRef.Name)
	destRef, _ := reference.GetReference(ctx, allocationID, newPath)
	if destRef != nil {
		return nil, common.NewError("invalid_parameters", "Invalid destination path. Object Already exists.")
	}

	allocationChange := &allocation.AllocationChange{}
	allocationChange.ConnectionID = connectionObj.ConnectionID
	allocationChange.Size = objectRef.Size
	allocationChange.Operation = constants.FileOperationCopy
	dfc := &allocation.CopyFileChange{ConnectionID: connectionObj.ConnectionID,
		AllocationID: connectionObj.AllocationID, DestPath: destPath}
	dfc.SrcPath = objectRef.Path
	connectionObj.Size += allocationChange.Size
	connectionObj.AddChange(allocationChange, dfc)

	err = connectionObj.Save(ctx)
	if err != nil {
		Logger.Error("Error in writing the connection meta data", zap.Error(err))
		return nil, common.NewError("connection_write_error", "Error writing the connection meta data")
	}

	result := &blobberhttp.UploadResult{}
	result.Filename = objectRef.Name
	result.Hash = objectRef.Hash
	result.MerkleRoot = objectRef.MerkleRoot
	result.Size = objectRef.Size

	return result, nil
}

func (fsh *StorageHandler) DeleteFile(ctx context.Context, r *http.Request, connectionObj *allocation.AllocationChangeCollector) (*blobberhttp.UploadResult, error) {
	path := r.FormValue("path")
	if path == "" {
		return nil, common.NewError("invalid_parameters", "Invalid path")
	}

	fileRef, _ := reference.GetReference(ctx, connectionObj.AllocationID, path)
	_ = ctx.Value(constants.ContextKeyClientKey).(string)
	if fileRef != nil {
		deleteSize := fileRef.Size

		allocationChange := &allocation.AllocationChange{}
		allocationChange.ConnectionID = connectionObj.ConnectionID
		allocationChange.Size = 0 - deleteSize
		allocationChange.Operation = constants.FileOperationDelete
		dfc := &allocation.DeleteFileChange{ConnectionID: connectionObj.ConnectionID,
			AllocationID: connectionObj.AllocationID, Name: fileRef.Name,
			Hash: fileRef.Hash, Path: fileRef.Path, Size: deleteSize}

		connectionObj.Size += allocationChange.Size
		connectionObj.AddChange(allocationChange, dfc)

		result := &blobberhttp.UploadResult{}
		result.Filename = fileRef.Name
		result.Hash = fileRef.Hash
		result.MerkleRoot = fileRef.MerkleRoot
		result.Size = fileRef.Size

		return result, nil
	}

	return nil, common.NewError("invalid_file", "File does not exist at path")
}

func (fsh *StorageHandler) CreateDir(ctx context.Context, r *http.Request) (*blobberhttp.UploadResult, error) {
	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	clientID := ctx.Value(constants.ContextKeyClient).(string)

	allocationObj, err := fsh.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	valid, err := verifySignatureFromRequest(allocationTx, r.Header.Get(common.ClientSignatureHeader), allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	allocationID := allocationObj.ID

	if clientID == "" {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
	}

	dirPath := r.FormValue("dir_path")
	if dirPath == "" {
		return nil, common.NewError("invalid_parameters", "Invalid dir path passed")
	}

	exisitingRef := fsh.checkIfFileAlreadyExists(ctx, allocationID, dirPath)
	if allocationObj.OwnerID != clientID && allocationObj.PayerID != clientID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
	}

	if exisitingRef != nil {
		return nil, common.NewError("duplicate_file", "File at path already exists")
	}

	connectionID := r.FormValue("connection_id")
	if connectionID == "" {
		return nil, common.NewError("invalid_parameters", "Invalid connection id passed")
	}

	connectionObj, err := allocation.GetAllocationChanges(ctx, connectionID, allocationID, clientID)
	if err != nil {
		return nil, common.NewError("meta_error", "Error reading metadata for connection")
	}

	mutex := lock.GetMutex(connectionObj.TableName(), connectionID)
	mutex.Lock()
	defer mutex.Unlock()

	allocationChange := &allocation.AllocationChange{}
	allocationChange.ConnectionID = connectionObj.ConnectionID
	allocationChange.Size = 0
	allocationChange.Operation = constants.FileOperationCreateDir
	connectionObj.Size += allocationChange.Size
	var formData allocation.NewFileChange
	formData.Filename = dirPath
	formData.Path = dirPath
	formData.AllocationID = allocationID
	formData.ConnectionID = connectionID
	formData.ActualHash = "-"
	formData.ActualSize = 1

	connectionObj.AddChange(allocationChange, &formData)

	err = connectionObj.ApplyChanges(ctx, "/")
	if err != nil {
		return nil, err
	}

	result := &blobberhttp.UploadResult{}
	result.Filename = dirPath
	result.Hash = ""
	result.MerkleRoot = ""
	result.Size = 0

	return result, nil
}

//WriteFile stores the file into the blobber files system from the HTTP request
func (fsh *StorageHandler) WriteFile(ctx context.Context, r *http.Request) (*blobberhttp.UploadResult, error) {
	if r.Method == "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used for the upload URL. Use multi-part form POST / PUT / DELETE / PATCH instead")
	}

	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	clientID := ctx.Value(constants.ContextKeyClient).(string)

	allocationObj, err := fsh.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	allocationID := allocationObj.ID
	fileOperation := getFileOperation(r)
	existingFileRef := getExistingFileRef(fsh, ctx, r, allocationObj, fileOperation)
	isCollaborator := existingFileRef != nil && reference.IsACollaborator(ctx, existingFileRef.ID, clientID)
	publicKey := allocationObj.OwnerPublicKey

	if isCollaborator {
		publicKey = ctx.Value(constants.ContextKeyClientKey).(string)
	}

	valid, err := verifySignatureFromRequest(allocationTx, r.Header.Get(common.ClientSignatureHeader), publicKey)

	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	if allocationObj.IsImmutable {
		return nil, common.NewError("immutable_allocation", "Cannot write to an immutable allocation")
	}

	if clientID == "" {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
	}

	if err := r.ParseMultipartForm(FormFileParseMaxMemory); err != nil {
		Logger.Info("Error Parsing the request", zap.Any("error", err))
		return nil, common.NewError("request_parse_error", err.Error())
	}

	connectionID := r.FormValue("connection_id")
	if connectionID == "" {
		return nil, common.NewError("invalid_parameters", "Invalid connection id passed")
	}

	cmd := createFileCommand(r)

	err = cmd.IsAuthorized(ctx, r, allocationObj, clientID)

	if err != nil {
		return nil, err
	}

	connectionObj, err := allocation.GetAllocationChanges(ctx, connectionID, allocationID, clientID)
	if err != nil {
		return nil, common.NewError("meta_error", "Error reading metadata for connection")
	}

	mutex := lock.GetMutex(connectionObj.TableName(), connectionID)
	mutex.Lock()
	defer mutex.Unlock()

	result, err := cmd.ProcessContent(ctx, r, allocationObj, connectionObj)

	if err != nil {
		return nil, err
	}

	err = cmd.ProcessThumbnail(ctx, r, allocationObj, connectionObj)

	if err != nil {
		return nil, err
	}

	err = cmd.UpdateChange(ctx, connectionObj)

	if err != nil {
		Logger.Error("Error in writing the connection meta data", zap.Error(err))
		return nil, common.NewError("connection_write_error", err.Error()) //"Error writing the connection meta data")
	}

	return &result, nil
}

func getFormFieldName(mode string) string {
	return "uploadMeta"
	//	formField := "uploadMeta"
	// if mode == constants.FileOperationUpdate {
	// 	//formField = "updateMeta"
	// }

	//return formField
}

func getFileOperation(r *http.Request) string {
	mode := constants.FileOperationInsert
	if r.Method == "PUT" {
		mode = constants.FileOperationUpdate
	} else if r.Method == "DELETE" {
		mode = constants.FileOperationDelete
	}

	return mode
}

func getExistingFileRef(fsh *StorageHandler, ctx context.Context, r *http.Request, allocationObj *allocation.Allocation, fileOperation string) *reference.Ref {
	if fileOperation == constants.FileOperationInsert || fileOperation == constants.FileOperationUpdate {
		var formData allocation.UpdateFileChanger
		uploadMetaString := r.FormValue(getFormFieldName(fileOperation))
		err := json.Unmarshal([]byte(uploadMetaString), &formData)

		if err == nil {
			return fsh.checkIfFileAlreadyExists(ctx, allocationObj.ID, formData.Path)
		}
	}
	return nil
}
