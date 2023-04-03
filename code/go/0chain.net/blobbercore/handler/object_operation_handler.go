package handler

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"

	"github.com/0chain/gosdk/constants"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/lock"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"

	"gorm.io/gorm"

	"go.uber.org/zap"

	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

const (
	// EncryptionHeaderSize encryption header size in chunk: PRE.MessageChecksum(128)"+PRE.OverallChecksum(128)
	EncryptionHeaderSize = 128 + 128
	// ReEncryptionHeaderSize re-encryption header size in chunk
	ReEncryptionHeaderSize = 256
)

func readPreRedeem(
	ctx context.Context, alloc *allocation.Allocation,
	numBlocks, pendNumBlocks int64, payerID string) (err error) {

	if numBlocks == 0 {
		return
	}

	// check out read pool tokens if read_price > 0
	var (
		db        = datastore.GetStore().GetTransaction(ctx)
		blobberID = node.Self.ID
	)

	if alloc.GetRequiredReadBalance(blobberID, numBlocks) <= 0 {
		return // skip if read price is zero
	}

	readPoolBalance, err := allocation.GetReadPoolsBalance(db, payerID)
	if err != nil {
		return common.NewError("read_pre_redeem", "database error while reading read pools balance")
	}

	requiredBalance := alloc.GetRequiredReadBalance(blobberID, numBlocks+pendNumBlocks)
	if float64(readPoolBalance) < requiredBalance {
		rp, err := allocation.RequestReadPoolStat(payerID)
		if err != nil {
			return common.NewErrorf("read_pre_redeem", "can't request read pools from sharders: %v", err)
		}

		rp.ClientID = payerID
		err = allocation.UpdateReadPool(db, rp)
		if err != nil {
			return common.NewErrorf("read_pre_redeem", "can't save requested read pools: %v", err)
		}

		readPoolBalance = rp.Balance

		if float64(readPoolBalance) < requiredBalance {
			return common.NewError("read_pre_redeem",
				"not enough tokens in client's read pools associated with the allocation->blobber")
		}
	}

	return
}

func writePreRedeem(ctx context.Context, alloc *allocation.Allocation, writeMarker *writemarker.WriteMarker, payerID string) (err error) {
	// check out read pool tokens if read_price > 0
	var (
		db              = datastore.GetStore().GetTransaction(ctx)
		blobberID       = node.Self.ID
		requiredBalance = alloc.GetRequiredWriteBalance(blobberID, writeMarker.Size, writeMarker.Timestamp)
		wp              *allocation.WritePool
	)

	if writeMarker.Size <= 0 || requiredBalance <= 0 {
		return
	}

	writePoolBalance, err := allocation.GetWritePoolsBalance(db, alloc.ID)
	if err != nil {
		Logger.Error("write_pre_redeem:get_write_pools_balance", zap.Error(err), zap.String("allocation_id", alloc.ID))
		return common.NewError("write_pre_redeem", "database error while getting write pool balance")
	}

	pendingWriteSize, err := allocation.GetPendingWrite(db, payerID, alloc.ID)
	if err != nil {
		Logger.Error("write_pre_redeem:get_pending_write", zap.Error(err), zap.String("allocation_id", alloc.ID), zap.String("payer_id", payerID))
		return common.NewError("write_pre_redeem", "database error while getting pending writes")
	}

	requiredBalance = alloc.GetRequiredWriteBalance(blobberID, pendingWriteSize+writeMarker.Size, writeMarker.Timestamp)

	if writePoolBalance < requiredBalance {
		wp, err = allocation.RequestWritePool(alloc.ID)
		if err != nil {
			return common.NewErrorf("write_pre_redeem", "can't request write pools from sharders: %v", err)
		}

		err = allocation.SetWritePool(db, alloc.ID, wp)
		if err != nil {
			return common.NewErrorf("write_pre_redeem", "can't save requested write pools: %v", err)
		}

		writePoolBalance += wp.Balance
	}

	if writePoolBalance < requiredBalance {
		return common.NewErrorf("write_pre_redeem", "not enough "+
			"tokens in write pools (client -> allocation ->  blobber)"+
			"(%s -> %s -> %s), available balance %d, required balance %d", payerID,
			alloc.ID, writeMarker.BlobberID, writePoolBalance, requiredBalance)
	}

	if err := allocation.AddToPending(db, payerID, alloc.ID, writeMarker.Size); err != nil {
		Logger.Error(err.Error())
		return common.NewErrorf("write_pre_redeem", "can't save pending writes in DB")

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

	key := clientID + ":" + alloc.ID
	lock, isNewLock := readmarker.ReadmarkerMapLock.GetLock(key)
	if !isNewLock {
		return nil, common.NewErrorf("lock_exists", fmt.Sprintf("lock exists for key: %v", key))
	}

	lock.Lock()
	defer lock.Unlock()

	dr, err := FromDownloadRequest(allocationTx, r)
	if err != nil {
		return nil, err
	}

	if dr.ReadMarker.ClientID != clientID { // We might well remove client id from request header
		return nil, common.NewError("invalid_client", "header clientID and readmarker clientID are different")
	}

	rmObj := new(readmarker.ReadMarkerEntity)
	rmObj.LatestRM = &dr.ReadMarker

	if err = rmObj.VerifyMarker(ctx, alloc); err != nil {
		return nil, common.NewErrorf("download_file", "invalid read marker, "+"failed to verify the read marker: %v", err)
	}

	fileref, err := reference.GetReferenceByLookupHash(ctx, alloc.ID, dr.PathHash)
	if err != nil {
		return nil, common.NewErrorf("download_file", "invalid file path: %v", err)
	}

	if fileref.Type != reference.FILE {
		return nil, common.NewErrorf("download_file", "path is not a file: %v", err)
	}

	isOwner := clientID == alloc.OwnerID

	var authToken *readmarker.AuthTicket
	var shareInfo *reference.ShareInfo

	if !isOwner {
		authTokenString := dr.AuthToken
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

		availableAt := shareInfo.AvailableAt.Unix()
		if common.Timestamp(availableAt) > common.Now() {
			return nil, common.NewErrorf("download_file", "the file is not available until: %v", shareInfo.AvailableAt.UTC().Format("2006-01-02T15:04:05"))
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

	if latestRM != nil && latestRM.ReadCounter+(dr.NumBlocks) > dr.ReadMarker.ReadCounter {
		latestRM.BlobberID = node.Self.ID
		return &blobberhttp.DownloadResponse{
			Success:      false,
			LatestRM:     latestRM,
			Path:         fileref.Path,
			AllocationID: fileref.AllocationID,
		}, common.NewError("stale_read_marker", "")
	}

	// check out read pool tokens if read_price > 0
	err = readPreRedeem(ctx, alloc, dr.NumBlocks, pendNumBlocks, clientID)
	if err != nil {
		return nil, common.NewErrorf("download_file", "pre-redeeming read marker: %v", err)
	}

	// reading is allowed
	var (
		downloadMode         = dr.DownloadMode
		fileDownloadResponse *filestore.FileDownloadResponse
		// respData             []byte
	)
	if downloadMode == DownloadContentThumb {
		rbi := &filestore.ReadBlockInput{
			AllocationID:  alloc.ID,
			FileSize:      fileref.ThumbnailSize,
			Hash:          fileref.ThumbnailHash,
			StartBlockNum: int(dr.BlockNum),
			NumBlocks:     int(dr.NumBlocks),
			IsThumbnail:   true,
		}

		fileDownloadResponse, err = filestore.GetFileStore().GetFileBlock(rbi)
		if err != nil {
			return nil, common.NewErrorf("download_file", "couldn't get thumbnail block: %v", err)
		}
	} else {
		rbi := &filestore.ReadBlockInput{
			AllocationID:   alloc.ID,
			FileSize:       fileref.Size,
			Hash:           fileref.ValidationRoot,
			StartBlockNum:  int(dr.BlockNum),
			NumBlocks:      int(dr.NumBlocks),
			VerifyDownload: dr.VerifyDownload,
		}
		fileDownloadResponse, err = filestore.GetFileStore().GetFileBlock(rbi)
		if err != nil {
			return nil, common.NewErrorf("download_file", "couldn't get file block: %v", err)
		}
	}

	err = readmarker.SaveLatestReadMarker(ctx, &dr.ReadMarker, 0, latestRM == nil)
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

	chunkData, err := chunkEncoder.Encode(int(fileref.ChunkSize), fileDownloadResponse.Data)
	if err != nil {
		return nil, err
	}

	fileDownloadResponse.Data = chunkData
	stats.FileBlockDownloaded(ctx, fileref.ID)
	return fileDownloadResponse, nil
}

func (fsh *StorageHandler) CommitWrite(ctx context.Context, r *http.Request) (*blobberhttp.CommitResult, error) {
	startTime := time.Now()
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

	if allocationObj.FileOptions == 0 {
		return nil, common.NewError("immutable_allocation", "Cannot write to an immutable allocation")
	}

	elapsedAllocation := time.Since(startTime)

	allocationID := allocationObj.ID

	connectionID, ok := common.GetField(r, "connection_id")
	if !ok {
		return nil, common.NewError("invalid_parameters", "Invalid connection id passed")
	}

	// Lock will compete with other CommitWrites and Challenge validation
	mutex := lock.GetMutex(allocationObj.TableName(), allocationID)
	mutex.Lock()
	defer mutex.Unlock()

	elapsedGetLock := time.Since(startTime) - elapsedAllocation
	connectionObj, err := allocation.GetAllocationChanges(ctx, connectionID, allocationID, clientID)
	if err != nil {
		// might be good to check if blobber already has stored writemarker
		return nil, common.NewErrorf("invalid_parameters",
			"Invalid connection id. Connection id was not found: %v", err)
	}
	if len(connectionObj.Changes) == 0 {
		return nil, common.NewError("invalid_parameters",
			"Invalid connection id. Connection does not have any changes.")
	}

	elapsedGetConnObj := time.Since(startTime) - elapsedAllocation - elapsedGetLock

	if clientID == "" || clientKey == "" {
		return nil, common.NewError("invalid_params", "Please provide clientID and clientKey")
	}

	if allocationObj.OwnerID != clientID || encryption.Hash(clientKeyBytes) != clientID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
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
	var latestWriteMarkerEntity *writemarker.WriteMarkerEntity
	if allocationObj.AllocationRoot == "" {
		latestWriteMarkerEntity = nil
	} else {
		latestWriteMarkerEntity, err = writemarker.GetWriteMarkerEntity(ctx,
			allocationObj.AllocationRoot)
		if err != nil {
			return nil, common.NewErrorf("latest_write_marker_read_error",
				"Error reading the latest write marker for allocation: %v", err)
		}
	}

	writemarkerEntity := &writemarker.WriteMarkerEntity{}
	writemarkerEntity.WM = writeMarker

	err = writemarkerEntity.VerifyMarker(ctx, allocationObj, connectionObj)
	if err != nil {
		result.AllocationRoot = allocationObj.AllocationRoot
		result.ErrorMessage = "Verification of write marker failed: " + err.Error()
		result.Success = false
		if latestWriteMarkerEntity != nil {
			result.WriteMarker = &latestWriteMarkerEntity.WM
		}
		return &result, common.NewError("write_marker_verification_failed", result.ErrorMessage)
	}

	elapsedVerifyWM := time.Since(startTime) - elapsedAllocation - elapsedGetLock - elapsedGetConnObj

	var clientIDForWriteRedeem = writeMarker.ClientID

	if err := writePreRedeem(ctx, allocationObj, &writeMarker, clientIDForWriteRedeem); err != nil {
		return nil, err
	}

	elapsedWritePreRedeem := time.Since(startTime) - elapsedAllocation - elapsedGetLock -
		elapsedGetConnObj - elapsedVerifyWM

	fileIDMetaStr := r.FormValue("file_id_meta")
	fileIDMeta := make(map[string]string, 0)
	err = json.Unmarshal([]byte(fileIDMetaStr), &fileIDMeta)
	if err != nil {
		return nil, common.NewError("unmarshall_error",
			fmt.Sprintf("Error while unmarshalling file ID meta data: %s", err.Error()))
	}

	err = connectionObj.ApplyChanges(
		ctx, writeMarker.AllocationRoot, writeMarker.Timestamp, fileIDMeta)
	if err != nil {
		return nil, err
	}

	elapsedApplyChanges := time.Since(startTime) - elapsedAllocation - elapsedGetLock -
		elapsedGetConnObj - elapsedVerifyWM - elapsedWritePreRedeem

	rootRef, err := reference.GetLimitedRefFieldsByPath(ctx, allocationID, "/", []string{"hash", "file_meta_hash"})
	if err != nil {
		return nil, err
	}
	allocationRoot := encryption.Hash(rootRef.Hash + ":" + strconv.FormatInt(int64(writeMarker.Timestamp), 10))
	fileMetaRoot := rootRef.FileMetaHash
	if allocationRoot != writeMarker.AllocationRoot {
		result.AllocationRoot = allocationObj.AllocationRoot
		if latestWriteMarkerEntity != nil {
			result.WriteMarker = &latestWriteMarkerEntity.WM
		}
		result.Success = false
		result.ErrorMessage = "Allocation root in the write marker does not match the calculated allocation root." +
			" Expected hash: " + allocationRoot
		return &result, common.NewError("allocation_root_mismatch", result.ErrorMessage)
	}

	if fileMetaRoot != writeMarker.FileMetaRoot {
		// result.AllocationRoot = allocationObj.AllocationRoot
		if latestWriteMarkerEntity != nil {
			result.WriteMarker = &latestWriteMarkerEntity.WM
		}
		result.Success = false
		result.ErrorMessage = "File meta root in the write marker does not match the calculated file meta root." +
			" Expected hash: " + fileMetaRoot + "; Got: " + writeMarker.FileMetaRoot
		return &result, common.NewError("file_meta_root_mismatch", result.ErrorMessage)
	}

	writemarkerEntity.ConnectionID = connectionObj.ID
	writemarkerEntity.ClientPublicKey = clientKey

	err = writemarkerEntity.Save(ctx)
	if err != nil {
		return nil, common.NewError("write_marker_error", "Error persisting the write marker")
	}

	db := datastore.GetStore().GetTransaction(ctx)
	allocationUpdates := make(map[string]interface{})
	allocationUpdates["blobber_size_used"] = gorm.Expr("blobber_size_used + ?", connectionObj.Size)
	allocationUpdates["used_size"] = gorm.Expr("used_size + ?", connectionObj.Size)
	allocationUpdates["allocation_root"] = allocationRoot
	allocationUpdates["file_meta_root"] = fileMetaRoot
	allocationUpdates["is_redeem_required"] = true

	err = db.Model(allocationObj).Updates(allocationUpdates).Error
	if err != nil {
		return nil, common.NewError("allocation_write_error", "Error persisting the allocation object")
	}
	err = connectionObj.CommitToFileStore(ctx)
	if err != nil {
		if !errors.Is(common.ErrFileWasDeleted, err) {
			return nil, common.NewError("file_store_error", "Error committing to file store. "+err.Error())
		}
	}

	result.Changes = connectionObj.Changes

	connectionObj.DeleteChanges(ctx)

	db.Model(connectionObj).Updates(allocation.AllocationChangeCollector{Status: allocation.CommittedConnection})

	result.AllocationRoot = allocationObj.AllocationRoot
	result.WriteMarker = &writeMarker
	result.Success = true
	result.ErrorMessage = ""
	commitOperation := connectionObj.Changes[0].Operation
	input := connectionObj.Changes[0].Input

	//Delete connection object and its changes
	for _, c := range connectionObj.Changes {
		db.Delete(c)
	}

	// db.Delete(connectionObj)

	connectionObj.Changes = []*allocation.AllocationChange{}
	connectionObj.AllocationChanges = []allocation.AllocationChangeProcessor{}

	for _, c := range result.Changes {

		switch c.Operation {
		case constants.FileOperationInsert:
			acp := new(allocation.UploadFileChanger)
			if err := json.Unmarshal([]byte(c.Input), acp); err != nil {
				Logger.Error("AllocationChangeCollector_unmarshal", zap.Error(err))
				break
			}
			if !acp.IsTemp {
				break
			}
			acp.IsTemp = false
			newChange := *c
			connectionObj.AddChange(&newChange, acp)
		case constants.FileOperationUpdate:
			acp := new(allocation.UpdateFileChanger)
			if err := json.Unmarshal([]byte(c.Input), acp); err != nil {
				Logger.Error("AllocationChangeCollector_unmarshal", zap.Error(err))
				break
			}
			if !acp.IsTemp {
				break
			}
			acp.IsTemp = false
			newChange := *c
			connectionObj.AddChange(&newChange, acp)
		}

	}

	if len(connectionObj.Changes) > 0 {
		err = connectionObj.Save(ctx)
		if err != nil {
			Logger.Error("Error in writing the connection meta data", zap.Error(err))
			return nil, common.NewError("connection_write_error", "Error writing the connection meta data")
		}
	} else {
		db.Delete(connectionObj)
	}

	Logger.Info("[commit]"+commitOperation,
		zap.String("alloc_id", allocationID),
		zap.String("input", input),
		zap.Duration("get_alloc", elapsedAllocation),
		zap.Duration("get-lock", elapsedGetLock),
		zap.Duration("get-conn-obj", elapsedGetConnObj),
		zap.Duration("verify-wm", elapsedVerifyWM),
		zap.Duration("write-pre-redeem", elapsedWritePreRedeem),
		zap.Duration("apply-changes", elapsedApplyChanges),
		zap.Duration("total", time.Since(startTime)),
	)
	return &result, nil
}

func (fsh *StorageHandler) RenameObject(ctx context.Context, r *http.Request) (interface{}, error) {
	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	if !allocationObj.CanRename() {
		return nil, common.NewError("prohibited_allocation_file_options", "Cannot rename data in this allocation.")
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

	objectRef, err := reference.GetLimitedRefFieldsByLookupHash(ctx, allocationID, pathHash, []string{"id", "name", "path", "hash", "size", "validation_root", "fixed_merkle_root"})

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	if objectRef.Path == "/" {
		return nil, common.NewError("invalid_operation", "cannot rename root path")
	}

	allocationChange := &allocation.AllocationChange{}
	allocationChange.ConnectionID = connectionObj.ID
	allocationChange.Size = 0
	allocationChange.Operation = constants.FileOperationRename
	dfc := &allocation.RenameFileChange{ConnectionID: connectionObj.ID,
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
	result.ValidationRoot = objectRef.ValidationRoot
	result.FixedMerkleRoot = objectRef.FixedMerkleRoot
	result.Size = objectRef.Size

	return result, nil
}

func (fsh *StorageHandler) CopyObject(ctx context.Context, r *http.Request) (interface{}, error) {

	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	if !allocationObj.CanCopy() {
		return nil, common.NewError("prohibited_allocation_file_options", "Cannot copy data from this allocation.")
	}

	valid, err := verifySignatureFromRequest(allocationTx, r.Header.Get(common.ClientSignatureHeader), allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
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

	objectRef, err := reference.GetLimitedRefFieldsByLookupHash(ctx, allocationID, pathHash, []string{"id", "name", "path", "hash", "size", "validation_root", "fixed_merkle_root"})

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}
	newPath := filepath.Join(destPath, objectRef.Name)
	paths, err := common.GetParentPaths(newPath)
	if err != nil {
		return nil, err
	}

	paths = append(paths, newPath)

	refs, err := reference.GetRefsTypeFromPaths(ctx, allocationID, paths)
	if err != nil {
		Logger.Error("Database error", zap.Error(err))
		return nil, common.NewError("database_error", fmt.Sprintf("Got db error while getting refs for %v", paths))
	}

	for _, ref := range refs {
		switch ref.Path {
		case newPath:
			return nil, common.NewError("invalid_parameters", "Invalid destination path. Object Already exists.")
		default:
			if ref.Type == reference.FILE {
				return nil, common.NewError("invalid_path", fmt.Sprintf("%v is of file type", ref.Path))
			}
		}
	}

	allocationChange := &allocation.AllocationChange{}
	allocationChange.ConnectionID = connectionObj.ID
	allocationChange.Size = objectRef.Size
	allocationChange.Operation = constants.FileOperationCopy
	dfc := &allocation.CopyFileChange{ConnectionID: connectionObj.ID,
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
	result.ValidationRoot = objectRef.ValidationRoot
	result.FixedMerkleRoot = objectRef.FixedMerkleRoot
	result.Size = objectRef.Size
	return result, nil
}

func (fsh *StorageHandler) MoveObject(ctx context.Context, r *http.Request) (interface{}, error) {

	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	if !allocationObj.CanMove() {
		return nil, common.NewError("prohibited_allocation_file_options", "Cannot move data in this allocation.")
	}

	valid, err := verifySignatureFromRequest(
		allocationTx, r.Header.Get(common.ClientSignatureHeader), allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
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

	objectRef, err := reference.GetLimitedRefFieldsByLookupHash(
		ctx, allocationID, pathHash, []string{"id", "name", "path", "hash", "size", "validation_root", "fixed_merkle_root"})

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}
	newPath := filepath.Join(destPath, objectRef.Name)
	paths, err := common.GetParentPaths(newPath)
	if err != nil {
		return nil, err
	}

	paths = append(paths, newPath)

	refs, err := reference.GetRefsTypeFromPaths(ctx, allocationID, paths)
	if err != nil {
		Logger.Error("Database error", zap.Error(err))
		return nil, common.NewError("database_error", fmt.Sprintf("Got db error while getting refs for %v", paths))
	}

	for _, ref := range refs {
		switch ref.Path {
		case newPath:
			return nil, common.NewError("invalid_parameters", "Invalid destination path. Object Already exists.")
		default:
			if ref.Type == reference.FILE {
				return nil, common.NewError("invalid_path", fmt.Sprintf("%v is of file type", ref.Path))
			}
		}
	}

	allocationChange := &allocation.AllocationChange{}
	allocationChange.ConnectionID = connectionObj.ID
	allocationChange.Size = 0
	allocationChange.Operation = constants.FileOperationMove
	dfc := &allocation.MoveFileChange{
		ConnectionID: connectionObj.ID,
		AllocationID: connectionObj.AllocationID,
		SrcPath:      objectRef.Path,
		DestPath:     destPath,
	}
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
	result.ValidationRoot = objectRef.ValidationRoot
	result.FixedMerkleRoot = objectRef.FixedMerkleRoot
	result.Size = objectRef.Size
	return result, nil
}

func (fsh *StorageHandler) DeleteFile(ctx context.Context, r *http.Request, connectionObj *allocation.AllocationChangeCollector) (*blobberhttp.UploadResult, error) {

	path := r.FormValue("path")
	if path == "" {
		return nil, common.NewError("invalid_parameters", "Invalid path")
	}
	fileRef, err := reference.GetLimitedRefFieldsByPath(ctx, connectionObj.AllocationID, path,
		[]string{"path", "name", "size", "hash", "validation_root", "fixed_merkle_root"})

	if err != nil {
		Logger.Error("invalid_file", zap.Error(err))
	}
	_ = ctx.Value(constants.ContextKeyClientKey).(string)
	if fileRef != nil {
		deleteSize := fileRef.Size

		allocationChange := &allocation.AllocationChange{}
		allocationChange.ConnectionID = connectionObj.ID
		allocationChange.Size = 0 - deleteSize
		allocationChange.Operation = constants.FileOperationDelete
		dfc := &allocation.DeleteFileChange{ConnectionID: connectionObj.ID,
			AllocationID: connectionObj.AllocationID, Name: fileRef.Name,
			Hash: fileRef.Hash, Path: fileRef.Path, Size: deleteSize}

		connectionObj.Size += allocationChange.Size
		connectionObj.AddChange(allocationChange, dfc)

		result := &blobberhttp.UploadResult{}
		result.Filename = fileRef.Name
		result.Hash = fileRef.Hash
		result.ValidationRoot = fileRef.ValidationRoot
		result.FixedMerkleRoot = fileRef.FixedMerkleRoot
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

	exisitingRef, err := fsh.checkIfFileAlreadyExists(ctx, allocationID, dirPath)
	if err != nil {
		Logger.Error("Error file reference", zap.Error(err))
	}

	result := &blobberhttp.UploadResult{
		Filename: dirPath,
	}

	if exisitingRef != nil {
		// target directory exists, return StatusOK
		if exisitingRef.Type == reference.DIRECTORY {
			return nil, common.NewError("directory_exists", "Directory already exists`")
		}

		return nil, common.NewError("duplicate_file", "File at path already exists")
	}
	if !filepath.IsAbs(dirPath) {
		return nil, common.NewError("invalid_path", fmt.Sprintf("%v is not absolute path", dirPath))
	}

	if clientID != allocationObj.OwnerID {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
	}

	if err := validateParentPathType(ctx, allocationID, dirPath); err != nil {
		return nil, err
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
	allocationChange.ConnectionID = connectionObj.ID
	allocationChange.Size = 0
	allocationChange.Operation = constants.FileOperationCreateDir
	connectionObj.Size += allocationChange.Size
	var newDir allocation.NewDir
	newDir.ConnectionID = connectionID
	newDir.Path = dirPath
	newDir.AllocationID = allocationID

	connectionObj.AddChange(allocationChange, &newDir)
	if err != nil {
		return nil, err
	}

	err = connectionObj.Save(ctx)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// WriteFile stores the file into the blobber files system from the HTTP request
func (fsh *StorageHandler) WriteFile(ctx context.Context, r *http.Request) (*blobberhttp.UploadResult, error) {

	startTime := time.Now()

	if r.Method == "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used for the upload URL. Use multi-part form POST / PUT / DELETE / PATCH instead")
	}

	allocationTx := ctx.Value(constants.ContextKeyAllocation).(string)
	clientID := ctx.Value(constants.ContextKeyClient).(string)

	allocationObj, err := fsh.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	elapsedAllocation := time.Since(startTime)

	if r.Method == http.MethodPost && !allocationObj.CanUpload() {
		return nil, common.NewError("prohibited_allocation_file_options", "Cannot upload data to this allocation.")
	}

	if r.Method == http.MethodPut && !allocationObj.CanUpdate() {
		return nil, common.NewError("prohibited_allocation_file_options", "Cannot update data in this allocation.")
	}

	if r.Method == http.MethodDelete && !allocationObj.CanDelete() {
		return nil, common.NewError("prohibited_allocation_file_options", "Cannot delete data in this allocation.")
	}

	st := time.Now()
	allocationID := allocationObj.ID
	cmd := createFileCommand(r)
	err = cmd.IsValidated(ctx, r, allocationObj, clientID)

	if err != nil {
		return nil, err
	}

	elapsedValidate := time.Since(st)
	st = time.Now()

	publicKey := allocationObj.OwnerPublicKey

	valid, err := verifySignatureFromRequest(allocationTx, r.Header.Get(common.ClientSignatureHeader), publicKey)

	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	if clientID == "" {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
	}

	connectionID, ok := common.GetField(r, "connection_id")
	if !ok {
		return nil, common.NewError("invalid_parameters", "Invalid connection id passed")
	}

	elapsedRef := time.Since(st)
	st = time.Now()
	connectionObj, err := allocation.GetAllocationChanges(ctx, connectionID, allocationID, clientID)
	if err != nil {
		return nil, common.NewError("meta_error", "Error reading metadata for connection")
	}

	Logger.Info(fmt.Sprintf("[upload] Acquiring lock for allocation: %s, connection: %s", allocationID, connectionID))
	mutex := lock.GetMutex(connectionObj.TableName(), connectionID)

	Logger.Info(fmt.Sprintf("[upload] Locking for allocation: %s, connection: %s", allocationID, connectionID))

	mutex.Lock()
	Logger.Info(fmt.Sprintf("[upload] Acquired lock for allocation: %s, connection: %s", allocationID, connectionID))

	defer func() {
		Logger.Info(fmt.Sprintf("[upload] Unlocking lock for allocation: %s, connection: %s", allocationID, connectionID))
		mutex.Unlock()
		Logger.Info(fmt.Sprintf("[upload] Unlocked lock for allocation: %s, connection: %s", allocationID, connectionID))
	}()

	elapsedAllocationChanges := time.Since(st)

	Logger.Info(fmt.Sprintf("[upload] Processing content for allocation %s, connection: %s", allocationID, connectionID))
	st = time.Now()
	result, err := cmd.ProcessContent(ctx, r, allocationObj, connectionObj)

	if err != nil {
		return nil, err
	}
	Logger.Info(fmt.Sprintf("[upload] Content processed for allocation: %s, connection: %s", allocationID, connectionID))

	err = cmd.ProcessThumbnail(ctx, r, allocationObj, connectionObj)

	if err != nil {
		return nil, err
	}

	elapsedProcess := time.Since(st)
	st = time.Now()
	err = cmd.UpdateChange(ctx, connectionObj)

	if err != nil {
		Logger.Error("Error in writing the connection meta data", zap.Error(err))
		return nil, common.NewError("connection_write_error", err.Error()) //"Error writing the connection meta data")
	}

	elapsedUpdateChange := time.Since(st)

	Logger.Info("[upload]elapsed",
		zap.String("alloc_id", allocationID),
		zap.String("file", cmd.GetPath()),
		zap.Duration("get_alloc", elapsedAllocation),
		zap.Duration("validate", elapsedValidate),
		zap.Duration("ref", elapsedRef),
		zap.Duration("load_changes", elapsedAllocationChanges),
		zap.Duration("process", elapsedProcess),
		zap.Duration("update_changes", elapsedUpdateChange),
		zap.Duration("total", time.Since(startTime)),
	)

	return &result, nil
}
