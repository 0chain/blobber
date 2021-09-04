package handler

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"github.com/pkg/errors"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/util"
	zencryption "github.com/0chain/gosdk/zboxcore/encryption"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/constants"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/lock"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	zfileref "github.com/0chain/gosdk/zboxcore/fileref"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

var (
	authorisationError = errors.New("Authorisation Error")
	invalidRequest = errors.New("Invalid Request")
	invalidParameters = errors.New("Invalid Parameters")
	immutableAllocation = errors.New("Immutable Allocation")
)

func readPreRedeem(ctx context.Context, alloc *allocation.Allocation,
	numBlocks, pendNumBlocks int64, payerID string) (err error) {

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

func writePreRedeem(ctx context.Context, alloc *allocation.Allocation,
	writeMarker *writemarker.WriteMarker, payerID string) (err error) {

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

func (fsh *StorageHandler) DownloadFile(
	ctx context.Context,
	r *http.Request,
) (resp interface{}, err error) {
	if r.Method == "GET" {
		return nil, common.NewError("download_file",
			"invalid method used (GET), use POST instead")
	}

	// get client and allocation ids
	var (
		clientID     = ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
		allocationTx = ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
		_            = ctx.Value(constants.CLIENT_KEY_CONTEXT_KEY).(string) // runtime type check
		alloc        *allocation.Allocation
	)

	// check client
	if len(clientID) == 0 {
		return nil, common.NewError("download_file", "invalid client")
	}

	// get and check allocation
	alloc, err = fsh.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewErrorf("download_file",
			"invalid allocation id passed: %v", err)
	}

	// get and parse file params
	if err = r.ParseMultipartForm(FORM_FILE_PARSE_MAX_MEMORY); nil != err {
		Logger.Info("download_file - request_parse_error", zap.Error(err))
		return nil, common.NewErrorf("download_file",
			"request_parse_error: %v", err)
	}

	pathHash, err := pathHashFromReq(r, alloc.ID)
	if err != nil {
		return nil, common.NewError("download_file", "invalid path")
	}

	var blockNumStr = r.FormValue("block_num")
	if len(blockNumStr) == 0 {
		return nil, common.NewError("download_file", "no block number")
	}

	var blockNum int64
	blockNum, err = strconv.ParseInt(blockNumStr, 10, 64)
	if err != nil || blockNum < 0 {
		return nil, common.NewError("download_file", "invalid block number")
	}

	var numBlocksStr = r.FormValue("num_blocks")
	if len(numBlocksStr) == 0 {
		numBlocksStr = "1"
	}

	var numBlocks int64
	numBlocks, err = strconv.ParseInt(numBlocksStr, 10, 64)
	if err != nil || numBlocks < 0 {
		return nil, common.NewError("download_file",
			"invalid number of blocks")
	}

	// get read marker
	var (
		readMarkerString = r.FormValue("read_marker")
		readMarker       = &readmarker.ReadMarker{}
	)
	err = json.Unmarshal([]byte(readMarkerString), &readMarker)
	if err != nil {
		return nil, common.NewErrorf("download_file", "invalid parameters, "+
			"error parsing the readmarker for download: %v", err)
	}

	var rmObj = &readmarker.ReadMarkerEntity{}
	rmObj.LatestRM = readMarker

	if err = rmObj.VerifyMarker(ctx, alloc); err != nil {
		return nil, common.NewErrorf("download_file", "invalid read marker, "+
			"failed to verify the read marker: %v", err)
	}

	// get file reference
	var fileref *reference.Ref
	fileref, err = reference.GetReferenceFromLookupHash(ctx, alloc.ID, pathHash)
	if err != nil {
		return nil, common.NewErrorf("download_file",
			"invalid file path: %v", err)
	}

	if fileref.Type != reference.FILE {
		return nil, common.NewErrorf("download_file",
			"path is not a file: %v", err)
	}

	// set payer: default
	var payerID = alloc.OwnerID

	// set payer: check for explicit allocation payer value
	if len(alloc.PayerID) > 0 {
		payerID = alloc.PayerID
	}

	// authorize file access
	var (
		isOwner        = clientID == alloc.OwnerID
		isRepairer     = clientID == alloc.RepairerID
		isCollaborator = reference.IsACollaborator(ctx, fileref.ID, clientID)
	)

	var authToken *readmarker.AuthTicket = nil

	if (!isOwner && !isRepairer && !isCollaborator) || len(r.FormValue("auth_token")) > 0 {
		var authTokenString = r.FormValue("auth_token")

		// check auth token
		if isAuthorized, err := fsh.verifyAuthTicket(ctx,
			authTokenString, alloc, fileref, clientID,
		); !isAuthorized {
			return nil, common.NewErrorf("download_file",
				"cannot verify auth ticket: %v", err)
		}

		authToken = &readmarker.AuthTicket{}
		err = json.Unmarshal([]byte(authTokenString), &authToken)
		if err != nil {
			return nil, common.NewErrorf("download_file",
				"error parsing the auth ticket for download: %v", err)
		}
		// set payer: check for command line payer flag (--rx_pay)
		if r.FormValue("rx_pay") == "true" {
			payerID = clientID
		}

		if json.Unmarshal([]byte(authTokenString), &readmarker.AuthTicket{}) != nil {
			return nil, common.NewErrorf("download_file",
				"error parsing the auth ticket for download: %v", err)
		}

		// we only check content hash if its authticket is referring to a file
		if authToken.RefType == zfileref.FILE && authToken.ContentHash != fileref.ContentHash {
			return nil, errors.New("content hash does not match the requested file content hash")
		}

		if authToken.RefType == zfileref.DIRECTORY {
			hashes := util.GetParentPathHashes(allocationTx, fileref.Path)
			found := false
			for _, hash := range hashes {
				if hash == authToken.FilePathHash {
					found = true
					break
				}
			}
			if !found {
				return nil, errors.New("auth ticket is not authorized to download file specified")
			}
		}
		readMarker.AuthTicket = datatypes.JSON(authTokenString)

		// check for file payer flag
		if fileAttrs, err := fileref.GetAttributes(); err != nil {
			return nil, common.NewErrorf("download_file",
				"error getting file attributes: %v", err)
		} else {
			if fileAttrs.WhoPaysForReads == common.WhoPays3rdParty {
				payerID = clientID
			}
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
		return nil, common.NewErrorf("download_file",
			"couldn't get read marker from DB: %v", err)
	}

	if rme != nil {
		latestRM = rme.LatestRM
		if pendNumBlocks, err = rme.PendNumBlocks(); err != nil {
			return nil, common.NewErrorf("download_file",
				"couldn't get number of blocks pending redeeming: %v", err)
		}
	}

	if latestRM != nil &&
		latestRM.ReadCounter+(numBlocks) != readMarker.ReadCounter {

		var response = &blobberhttp.DownloadResponse{
			Success:      false,
			LatestRM:     latestRM,
			Path:         fileref.Path,
			AllocationID: fileref.AllocationID,
		}
		return response, nil
	}

	// check out read pool tokens if read_price > 0
	err = readPreRedeem(ctx, alloc, numBlocks, pendNumBlocks, payerID)
	if err != nil {
		return nil, common.NewErrorf("download_file",
			"pre-redeeming read marker: %v", err)
	}

	// reading is allowed
	var (
		downloadMode = r.FormValue("content")
		respData     []byte
	)
	if len(downloadMode) > 0 && downloadMode == DOWNLOAD_CONTENT_THUMB {
		var fileData = &filestore.FileInputData{}
		fileData.Name = fileref.Name
		fileData.Path = fileref.Path
		fileData.Hash = fileref.ThumbnailHash
		fileData.OnCloud = fileref.OnCloud
		respData, err = filestore.GetFileStore().GetFileBlock(alloc.ID,
			fileData, blockNum, numBlocks)
		if err != nil {
			return nil, common.NewErrorf("download_file",
				"couldn't get thumbnail block: %v", err)
		}
	} else {
		var fileData = &filestore.FileInputData{}
		fileData.Name = fileref.Name
		fileData.Path = fileref.Path
		fileData.Hash = fileref.ContentHash
		fileData.OnCloud = fileref.OnCloud
		respData, err = filestore.GetFileStore().GetFileBlock(alloc.ID,
			fileData, blockNum, numBlocks)
		if err != nil {
			return nil, common.NewErrorf("download_file",
				"couldn't get file block: %v", err)
		}
	}

	readMarker.PayerID = payerID
	err = readmarker.SaveLatestReadMarker(ctx, readMarker, latestRM == nil)
	if err != nil {
		return nil, common.NewErrorf("download_file",
			"couldn't save latest read marker: %v", err)
	}

	if len(fileref.EncryptedKey) > 0 {
		if authToken == nil {
			return nil, errors.New("auth ticket is required to download encrypted file")
		}
		// check if client is authorized to download
		shareInfo, err := reference.GetShareInfo(
			ctx,
			readMarker.ClientID,
			authToken.FilePathHash,
		)
		if err != nil {
			return nil, errors.New("error during share info lookup in database" + err.Error())
		} else if shareInfo == nil || shareInfo.Revoked {
			return nil, errors.New("client does not have permission to download the file. share does not exist")
		}

		buyerEncryptionPublicKey := shareInfo.ClientEncryptionPublicKey
		encscheme := zencryption.NewEncryptionScheme()
		// reEncrypt does not require pub / private key,
		// we could probably make it a classless function

		if err := encscheme.Initialize(""); err != nil {
			return nil, err
		}
		if err := encscheme.InitForDecryption("filetype:audio", fileref.EncryptedKey); err != nil {
			return nil, err
		}

		totalSize := len(respData)
		result := []byte{}
		for i := 0; i < totalSize; i += reference.CHUNK_SIZE {
			encMsg := &zencryption.EncryptedMessage{}
			chunkData := respData[i:int64(math.Min(float64(i+reference.CHUNK_SIZE), float64(totalSize)))]

			encMsg.EncryptedData = chunkData[(2 * 1024):]

			headerBytes := chunkData[:(2 * 1024)]
			headerBytes = bytes.Trim(headerBytes, "\x00")
			headerString := string(headerBytes)

			headerChecksums := strings.Split(headerString, ",")
			if len(headerChecksums) != 2 {
				Logger.Error("Block has invalid header", zap.String("request Url", r.URL.String()))
				return nil, errors.New("Block has invalid header for request " + r.URL.String())
			}

			encMsg.MessageChecksum, encMsg.OverallChecksum = headerChecksums[0], headerChecksums[1]
			encMsg.EncryptedKey = encscheme.GetEncryptedKey()

			reEncMsg, err := encscheme.ReEncrypt(encMsg, shareInfo.ReEncryptionKey, buyerEncryptionPublicKey)
			if err != nil {
				return nil, err
			}

			encData, err := reEncMsg.Marshal()
			if err != nil {
				return nil, err
			}
			result = append(result, encData...)
		}
		respData = result
	}

	if err := stats.FileBlockDownloaded(ctx, fileref.ID); err != nil {
		return nil, err
	}

	return respData, nil
}

func (fsh *StorageHandler) CommitWrite(ctx context.Context, r *http.Request) (*blobberhttp.CommitResult, error) {

	if r.Method == "GET" {
		return nil, common.NewError("invalid_method", "Invalid method used for the upload URL. Use POST instead")
	}
	allocationTx := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	clientKey := ctx.Value(constants.CLIENT_KEY_CONTEXT_KEY).(string)
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
	if len(connectionID) == 0 {
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
		if change.Operation == allocation.UPDATE_OPERATION {
			updateFileChange := new(allocation.UpdateFileChange)
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
	}

	if len(clientID) == 0 || len(clientKey) == 0 {
		return nil, common.NewError("invalid_params", "Please provide clientID and clientKey")
	}

	if (allocationObj.OwnerID != clientID || encryption.Hash(clientKeyBytes) != clientID) && !isCollaborator {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	if err = r.ParseMultipartForm(FORM_FILE_PARSE_MAX_MEMORY); nil != err {
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
	if len(allocationObj.AllocationRoot) == 0 {
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

	if err = writePreRedeem(ctx, allocationObj, &writeMarker, clientIDForWriteRedeem); err != nil {
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

func (fsh *StorageHandler) RenameObject(ctx context.Context, request *blobbergrpc.RenameObjectRequest) (*blobbergrpc.RenameObjectResponse, error) {

	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	clientSign := ctx.Value(constants.CLIENT_SIGNATURE_HEADER_KEY).(string)

	allocationObj, err := fsh.verifyAllocation(ctx, request.Allocation, false)
	if err != nil {
		return nil, errors.Wrap(err,
			"invalid allocation id passed")
	}

	if allocationObj.IsImmutable {
		return nil, errors.Wrap(immutableAllocation,
			"can't rename an immutable allocation")
	}

	valid, err := verifySignatureFromRequest(request.Allocation, clientSign, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, errors.Wrap(err,
			"failed to verify signature for the request")
	}

	if request.NewName == "" || request.ConnectionId == ""{
		return nil, errors.Wrap(invalidRequest,
			"empty parameters passed in the request")
	}

	if request.PathHash == ""{
		if request.Path == "" {
			Logger.Error("Invalid request path passed in the request")
			return nil, errors.Wrap(invalidParameters,
				"invalid request path")
		}
		request.PathHash = reference.GetReferenceLookup(allocationObj.ID, request.Path)
	}

	if len(clientID) == 0 || allocationObj.OwnerID != clientID {
		return nil, errors.Wrap(authorisationError,
			"operation needs to be performed by the owner of the allocation")
	}

	connectionObj, err := allocation.GetAllocationChanges(ctx, request.ConnectionId, allocationObj.ID, clientID)
	if err != nil {
		return nil, errors.Wrap(err,
			"failed to read metadata for the connection")
	}

	mutex := lock.GetMutex(connectionObj.TableName(), request.ConnectionId)
	mutex.Lock()
	defer mutex.Unlock()

	objectRef, err := reference.GetReferenceFromLookupHash(ctx, allocationObj.ID, request.PathHash)

	if err != nil {
		return nil, errors.Wrap(err,
			"invalid file path passed")
	}

	allocationChange := &allocation.AllocationChange{
		ConnectionID: connectionObj.ConnectionID,
		Size: 0,
		Operation: allocation.RENAME_OPERATION,
	}

	dfc := &allocation.RenameFileChange{
		ConnectionID: connectionObj.ConnectionID,
		AllocationID: connectionObj.AllocationID,
		Path: objectRef.Path,
		NewName: request.NewName,
	}

	connectionObj.Size += allocationChange.Size
	connectionObj.AddChange(allocationChange, dfc)

	err = connectionObj.Save(ctx)
	if err != nil {
		Logger.Error("Error in writing the connection meta data", zap.Error(err))
		return nil, errors.Wrap(err,
			"failed to write the connection metadata in db")
	}

	result := &blobbergrpc.RenameObjectResponse{
		Filename:     request.NewName,
		Size:         objectRef.Size,
		ContentHash:  objectRef.Hash,
		MerkleRoot:   objectRef.MerkleRoot,
		UploadLength: 0,
		UploadOffset: 0,
	}

	return result, nil
}

func (fsh *StorageHandler) UpdateObjectAttributes(ctx context.Context,
	request *blobbergrpc.UpdateObjectAttributesRequest) (response *blobbergrpc.UpdateObjectAttributesResponse, err error) {

	var (
		// todo(kushthedude): generalise the allocation_context in the grpc metadata
		//allocTx  = request.Allocation
		clientID = ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)

		clientSign = ctx.Value(constants.CLIENT_SIGNATURE_HEADER_KEY).(string)
		alloc         *allocation.Allocation
	)

	if alloc, err = fsh.verifyAllocation(ctx, request.Allocation, false); err != nil {
		return nil, errors.Wrap(err,
			"failed to verify allocation")
	}

	valid, err := verifySignatureFromRequest(request.Allocation, clientSign, alloc.OwnerPublicKey)
	if !valid || err != nil {
		return nil, errors.Wrap(errors.New("Authorisation Error"),
			"failed to verify signature from request")
	}

	if alloc.IsImmutable {
		return nil, errors.Wrap(errors.New("Immutable Allocation Error"),
			"can't update data in an immutable allocation")
	}

	if clientID == "" || request.Attributes == "" || request.ConnectionId == "" {
		return nil, errors.Wrap(errors.New("Invalid Request"),
			"client ID or attributed not present in the request header")
	}

	var attrs = new(reference.Attributes)
	if err = json.Unmarshal([]byte(request.Attributes), attrs); err != nil {
		return nil, errors.Wrap(err,
			"failed to unmarshal attributes")
	}

	if request.PathHash == ""{
		if request.Path == "" {
			Logger.Error("Invalid request path passed in the request")
			return nil, errors.Wrapf(errors.New("invalid request parameters"), "invalid request path")
		}
		request.PathHash = reference.GetReferenceLookup(alloc.ID, request.Path)
	}

	if alloc.OwnerID != clientID {
		return nil, errors.Wrap(errors.New("Authorisation Error"),
			"operation needs to be performed by the owner of the allocation")
	}

	var conn *allocation.AllocationChangeCollector
	conn, err = allocation.GetAllocationChanges(ctx, request.ConnectionId, alloc.ID, clientID)
	if err != nil {
		return nil, errors.Wrap(err,
			"failed to get allocation changes")
	}

	var mutex = lock.GetMutex(conn.TableName(), request.ConnectionId)

	mutex.Lock()
	defer mutex.Unlock()

	var ref *reference.Ref
	ref, err = reference.GetReferenceFromLookupHash(ctx, alloc.ID, request.PathHash)
	if err != nil {
		return nil, errors.Wrap(err,
			"failed to get reference from hash")
	}

	var change = new(allocation.AllocationChange)
	change.ConnectionID = conn.ConnectionID
	change.Operation = allocation.UPDATE_ATTRS_OPERATION

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
		return nil, errors.Wrap(err,
			"error in writing the connection meta data")
	}

	var result blobbergrpc.UpdateObjectAttributesResponse
	result.WhoPaysForReads = int64(attrs.WhoPaysForReads)

	// return new attributes as result
	return &result, nil
}

func (fsh *StorageHandler) CopyObject(ctx context.Context, request *blobbergrpc.CopyObjectRequest) (*blobbergrpc.CopyObjectResponse, error) {

	clientSign := ctx.Value(constants.CLIENT_SIGNATURE_HEADER_KEY).(string)
	allocationObj, err := fsh.verifyAllocation(ctx, request.Allocation, false)

	if err != nil {
		return nil, errors.Wrap(err,
			"invalid allocation ID passed")
	}

	valid, err := verifySignatureFromRequest(request.Allocation, clientSign, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, errors.Wrap(invalidRequest,
			"invalid signature for the request")
	}

	if allocationObj.IsImmutable {
		return nil, errors.Wrap(immutableAllocation,
			"failed to copy data in immutable allocation")
	}

	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)

	if request.Dest == "" || request.ConnectionId == "" || clientID == "" {
		return nil, errors.Wrap(invalidParameters,
			"invalid request body passed for the operation")
	}

	if request.PathHash == ""{
		if request.Path == "" {
			Logger.Error("Invalid request path passed in the request")
			return nil, errors.Wrapf(invalidParameters,
				"invalid request path")
		}
		request.PathHash = reference.GetReferenceLookup(allocationObj.ID, request.Path)
	}

	if allocationObj.OwnerID != clientID {
		return nil, errors.Wrap(authorisationError,
			"operation can be performed by the owner of allocation")
	}

	connectionObj, err := allocation.GetAllocationChanges(ctx, request.ConnectionId, allocationObj.ID, clientID)
	if err != nil {
		return nil, errors.Wrap(err,
			"failed to read metadata for the connection")
	}

	mutex := lock.GetMutex(connectionObj.TableName(), request.ConnectionId)
	mutex.Lock()
	defer mutex.Unlock()

	objectRef, err := reference.GetReferenceFromLookupHash(ctx, allocationObj.ID, request.PathHash)

	if err != nil {
		return nil, errors.Wrap(err,
			"failed to get reference from pathHash")
	}
	newPath := filepath.Join(request.Dest, objectRef.Name)

	_, err = reference.GetReference(ctx, allocationObj.ID, newPath)
	//If any object is present in the path then we would get error as nil,
	//If there is no object present we would get an error of `Record not found` by gorm
	//changing the logic here as we should never ever ignore the error.
	if err == nil {
		return nil, errors.Wrap(invalidParameters,
			"object already exists in the passed path")
	}

	destRef, err := reference.GetReference(ctx, allocationObj.ID, request.Dest)
	if err != nil || destRef.Type != reference.DIRECTORY {
		return nil, errors.Wrap(invalidParameters,
			"invalid destination directory path provided")
	}

	allocationChange := &allocation.AllocationChange{
		ConnectionID: connectionObj.ConnectionID,
		Size: objectRef.Size,
		Operation: allocation.COPY_OPERATION,
	}

	dfc := &allocation.CopyFileChange{
		ConnectionID: connectionObj.ConnectionID,
		AllocationID: connectionObj.AllocationID,
		DestPath: request.Dest,
		SrcPath: objectRef.Path,
	}

	connectionObj.Size += allocationChange.Size
	connectionObj.AddChange(allocationChange, dfc)

	err = connectionObj.Save(ctx)

	if err != nil {
		Logger.Error("Error in writing the connection meta data", zap.Error(err))
		return nil, errors.Wrap(err, "failed to write the updated metadata in db")
	}

	result := &blobbergrpc.CopyObjectResponse{
		Filename: objectRef.Name,
		ContentHash: objectRef.Hash,
		MerkleRoot: objectRef.MerkleRoot,
		Size: objectRef.Size,
	}

	return result, nil
}

func (fsh *StorageHandler) DeleteFile(ctx context.Context, r *http.Request, connectionObj *allocation.AllocationChangeCollector) (*blobberhttp.UploadResult, error) {
	path := r.FormValue("path")
	if len(path) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid path")
	}

	fileRef, _ := reference.GetReference(ctx, connectionObj.AllocationID, path)
	_ = ctx.Value(constants.CLIENT_KEY_CONTEXT_KEY).(string)
	if fileRef != nil {
		deleteSize := fileRef.Size

		allocationChange := &allocation.AllocationChange{}
		allocationChange.ConnectionID = connectionObj.ConnectionID
		allocationChange.Size = 0 - deleteSize
		allocationChange.Operation = allocation.DELETE_OPERATION
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
	allocationTx := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)

	allocationObj, err := fsh.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	valid, err := verifySignatureFromRequest(allocationTx, r.Header.Get(common.ClientSignatureHeader), allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	allocationID := allocationObj.ID

	if len(clientID) == 0 {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
	}

	dirPath := r.FormValue("dir_path")
	if len(dirPath) == 0 {
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
	if len(connectionID) == 0 {
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
	allocationChange.Operation = allocation.CREATEDIR_OPERATION
	connectionObj.Size += allocationChange.Size
	var formData allocation.NewFileChange
	formData.Filename = dirPath
	formData.Path = dirPath
	formData.AllocationID = allocationID
	formData.ConnectionID = connectionID
	formData.ActualHash = "-"
	formData.ActualSize = 1

	connectionObj.AddChange(allocationChange, &formData)

	err = filestore.GetFileStore().CreateDir(dirPath)
	if err != nil {
		return nil, common.NewError("upload_error", "Failed to upload the file. "+err.Error())
	}

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
		return nil, common.NewError("invalid_method", "Invalid method used for the upload URL. Use multi-part form POST / PUT / DELETE instead")
	}

	allocationTx := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)

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
		publicKey = ctx.Value(constants.CLIENT_KEY_CONTEXT_KEY).(string)
	}

	valid, err := verifySignatureFromRequest(allocationTx, r.Header.Get(common.ClientSignatureHeader), publicKey)

	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	if allocationObj.IsImmutable {
		return nil, common.NewError("immutable_allocation", "Cannot write to an immutable allocation")
	}

	if len(clientID) == 0 {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
	}

	if err := r.ParseMultipartForm(FORM_FILE_PARSE_MAX_MEMORY); err != nil {
		Logger.Info("Error Parsing the request", zap.Any("error", err))
		return nil, common.NewError("request_parse_error", err.Error())
	}

	connectionID := r.FormValue("connection_id")
	if len(connectionID) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid connection id passed")
	}

	connectionObj, err := allocation.GetAllocationChanges(ctx, connectionID, allocationID, clientID)
	if err != nil {
		return nil, common.NewError("meta_error", "Error reading metadata for connection")
	}

	mutex := lock.GetMutex(connectionObj.TableName(), connectionID)
	mutex.Lock()
	defer mutex.Unlock()

	result := &blobberhttp.UploadResult{}

	if fileOperation == allocation.DELETE_OPERATION {
		if allocationObj.OwnerID != clientID && allocationObj.RepairerID != clientID {
			return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
		}
		result, err = fsh.DeleteFile(ctx, r, connectionObj)
		if err != nil {
			return nil, err
		}
	} else if fileOperation == allocation.INSERT_OPERATION || fileOperation == allocation.UPDATE_OPERATION {
		formField := getFormFieldName(fileOperation)
		var formData allocation.UpdateFileChange
		uploadMetaString := r.FormValue(formField)
		err = json.Unmarshal([]byte(uploadMetaString), &formData)
		if err != nil {
			return nil, common.NewError("invalid_parameters",
				"Invalid parameters. Error parsing the meta data for upload."+err.Error())
		}
		existingFileRefSize := int64(0)
		existingFileOnCloud := false
		if fileOperation == allocation.INSERT_OPERATION {
			if allocationObj.OwnerID != clientID && allocationObj.RepairerID != clientID {
				return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
			}

			if existingFileRef != nil {
				return nil, common.NewError("duplicate_file", "File at path already exists")
			}
		} else if fileOperation == allocation.UPDATE_OPERATION {
			if existingFileRef == nil {
				return nil, common.NewError("invalid_file_update", "File at path does not exist for update")
			}

			if allocationObj.OwnerID != clientID && allocationObj.RepairerID != clientID && !isCollaborator {
				return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner, collaborator or the payer of the allocation")
			}
		}

		if existingFileRef != nil {
			existingFileRefSize = existingFileRef.Size
			existingFileOnCloud = existingFileRef.OnCloud
		}

		origfile, _, err := r.FormFile("uploadFile")
		if err != nil {
			return nil, common.NewError("invalid_parameters", "Error Reading multi parts for file."+err.Error())
		}
		defer origfile.Close()

		thumbfile, thumbHeader, _ := r.FormFile("uploadThumbnailFile")
		thumbnailPresent := thumbHeader != nil
		if thumbnailPresent {
			defer thumbfile.Close()
		}

		fileInputData := &filestore.FileInputData{Name: formData.Filename, Path: formData.Path, OnCloud: existingFileOnCloud}
		fileOutputData, err := filestore.GetFileStore().WriteFile(allocationID, fileInputData, origfile, connectionObj.ConnectionID)
		if err != nil {
			return nil, common.NewError("upload_error", "Failed to upload the file. "+err.Error())
		}

		result.Filename = formData.Filename
		result.Hash = fileOutputData.ContentHash
		result.MerkleRoot = fileOutputData.MerkleRoot
		result.Size = fileOutputData.Size

		if len(formData.Hash) > 0 && formData.Hash != fileOutputData.ContentHash {
			return nil, common.NewError("content_hash_mismatch", "Content hash provided in the meta data does not match the file content")
		}
		if len(formData.MerkleRoot) > 0 && formData.MerkleRoot != fileOutputData.MerkleRoot {
			return nil, common.NewError("content_merkle_root_mismatch", "Merkle root provided in the meta data does not match the file content")
		}
		if fileOutputData.Size > config.Configuration.MaxFileSize {
			return nil, common.NewError("file_size_limit_exceeded", "Size for the given file is larger than the max limit")
		}

		formData.Hash = fileOutputData.ContentHash
		formData.MerkleRoot = fileOutputData.MerkleRoot
		formData.AllocationID = allocationID
		formData.Size = fileOutputData.Size

		allocationSize := fileOutputData.Size
		if thumbnailPresent {
			thumbInputData := &filestore.FileInputData{Name: thumbHeader.Filename, Path: formData.Path}
			thumbOutputData, err := filestore.GetFileStore().WriteFile(allocationID, thumbInputData, thumbfile, connectionObj.ConnectionID)
			if err != nil {
				return nil, common.NewError("upload_error", "Failed to upload the thumbnail. "+err.Error())
			}
			if len(formData.ThumbnailHash) > 0 && formData.ThumbnailHash != thumbOutputData.ContentHash {
				return nil, common.NewError("content_hash_mismatch", "Content hash provided in the meta data does not match the thumbnail content")
			}
			formData.ThumbnailHash = thumbOutputData.ContentHash
			formData.ThumbnailSize = thumbOutputData.Size
			formData.ThumbnailFilename = thumbInputData.Name
		}

		if allocationObj.BlobberSizeUsed+(allocationSize-existingFileRefSize) > allocationObj.BlobberSize {
			return nil, common.NewError("max_allocation_size", "Max size reached for the allocation with this blobber")
		}

		allocationChange := &allocation.AllocationChange{}
		allocationChange.ConnectionID = connectionObj.ConnectionID
		allocationChange.Size = allocationSize - existingFileRefSize
		allocationChange.Operation = fileOperation

		connectionObj.Size += allocationChange.Size
		if fileOperation == allocation.INSERT_OPERATION {
			connectionObj.AddChange(allocationChange, &formData.NewFileChange)
		} else if fileOperation == allocation.UPDATE_OPERATION {
			connectionObj.AddChange(allocationChange, &formData)
		}
	}
	err = connectionObj.Save(ctx)
	if err != nil {
		Logger.Error("Error in writing the connection meta data", zap.Error(err))
		return nil, common.NewError("connection_write_error", "Error writing the connection meta data")
	}

	return result, nil
}

func getFormFieldName(mode string) string {
	formField := "uploadMeta"
	if mode == allocation.UPDATE_OPERATION {
		formField = "updateMeta"
	}

	return formField
}

func getFileOperation(r *http.Request) string {
	mode := allocation.INSERT_OPERATION
	if r.Method == "PUT" {
		mode = allocation.UPDATE_OPERATION
	} else if r.Method == "DELETE" {
		mode = allocation.DELETE_OPERATION
	}

	return mode
}

func getExistingFileRef(fsh *StorageHandler, ctx context.Context, r *http.Request, allocationObj *allocation.Allocation, fileOperation string) *reference.Ref {
	if fileOperation == allocation.INSERT_OPERATION || fileOperation == allocation.UPDATE_OPERATION {
		var formData allocation.UpdateFileChange
		uploadMetaString := r.FormValue(getFormFieldName(fileOperation))
		err := json.Unmarshal([]byte(uploadMetaString), &formData)

		if err == nil {
			return fsh.checkIfFileAlreadyExists(ctx, allocationObj.ID, formData.Path)
		}
	}
	return nil
}
