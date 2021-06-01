package handler

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/convert"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"strconv"
)

func (b *blobberGRPCService) DownloadFile(ctx context.Context, r *blobbergrpc.DownloadFileRequest) (*blobbergrpc.DownloadFileResponse, error) {
	md := GetGRPCMetaDataFromCtx(ctx)
	ctx = setupGRPCHandlerContext(ctx, md, r.Allocation)

	clientID := md.Client
	if len(clientID) == 0 {
		return nil, common.NewError("download_file", "invalid client")
	}

	allocationTx := r.Allocation
	allocationObj, err := b.storageHandler.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}
	var allocationID = allocationObj.ID

	if len(clientID) == 0 || allocationObj.OwnerID != clientID {
		return nil, common.NewError(
			"invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	rxPay := r.RxPay == "true"
	pathHash := r.PathHash
	path := r.Path
	if len(pathHash) == 0 {
		if len(path) == 0 {
			return nil, common.NewError("invalid_parameters", "Invalid path")
		}
		pathHash = b.packageHandler.GetReferenceLookup(ctx, allocationObj.ID, path)
	}

	var blockNumStr = r.BlockNum
	if len(blockNumStr) == 0 {
		return nil, common.NewError("download_file", "no block number")
	}

	var blockNum int64
	blockNum, err = strconv.ParseInt(blockNumStr, 10, 64)
	if err != nil || blockNum < 0 {
		return nil, common.NewError("download_file", "invalid block number")
	}

	// we can add r.NumBlocks as int64 if len() validation is not required
	var numBlocksStr = r.NumBlocks
	if len(numBlocksStr) == 0 {
		numBlocksStr = "1"
	}

	var numBlocks int64
	numBlocks, err = strconv.ParseInt(numBlocksStr, 10, 64)
	if err != nil || numBlocks < 0 {
		return nil, common.NewError("download_file",
			"invalid number of blocks")
	}

	var (
		readMarkerString = r.ReadMarker
		readMarker       = &readmarker.ReadMarker{}
	)
	err = json.Unmarshal([]byte(readMarkerString), readMarker)
	if err != nil {
		return nil, common.NewErrorf("download_file", "invalid parameters, "+
			"error parsing the readmarker for download: %v", err)
	}

	var rmObj = &readmarker.ReadMarkerEntity{}
	rmObj.LatestRM = readMarker

	if err = b.packageHandler.VerifyReadMarker(ctx, rmObj, allocationObj); err != nil {
		return nil, common.NewErrorf("download_file", "invalid read marker, "+
			"failed to verify the read marker: %v", err)
	}

	var fileref *reference.Ref
	fileref, err = b.packageHandler.GetReferenceFromLookupHash(ctx, allocationID, pathHash)
	if err != nil {
		return nil, common.NewErrorf("download_file",
			"invalid file path: %v", err)
	}
	if fileref.Type != reference.FILE {
		return nil, common.NewErrorf("download_file",
			"path is not a file: %v", err)
	}

	var (
		authTokenString       = r.AuthToken
		clientIDForReadRedeem = clientID // default payer is client
		isACollaborator       = b.packageHandler.IsACollaborator(ctx, fileref.ID, clientID)
	)
	// Owner will pay for collaborator
	if isACollaborator {
		clientIDForReadRedeem = allocationObj.OwnerID
	}

	if (allocationObj.OwnerID != clientID &&
		allocationObj.PayerID != clientID &&
		!isACollaborator) || len(authTokenString) > 0 {

		var authTicketVerified bool
		authTicketVerified, err = b.storageHandler.verifyAuthTicket(ctx, r.AuthToken, allocationObj,
			fileref, clientID)
		if err != nil {
			return nil, common.NewErrorf("download_file",
				"verifying auth ticket: %v", err)
		}

		if !authTicketVerified {
			return nil, common.NewErrorf("download_file",
				"could not verify the auth ticket")
		}

		var authToken = &readmarker.AuthTicket{}
		err = json.Unmarshal([]byte(authTokenString), &authToken)
		if err != nil {
			return nil, common.NewErrorf("download_file",
				"error parsing the auth ticket for download: %v", err)
		}

		var attrs *reference.Attributes
		if attrs, err = fileref.GetAttributes(); err != nil {
			return nil, common.NewErrorf("download_file",
				"error getting file attributes: %v", err)
		}

		// if --rx_pay used 3rd_party pays
		if rxPay {
			clientIDForReadRedeem = clientID
		} else if attrs.WhoPaysForReads == common.WhoPaysOwner {
			clientIDForReadRedeem = allocationObj.OwnerID // owner pays
		}
		readMarker.AuthTicket = datatypes.JSON(authTokenString)
	}

	var (
		rme           *readmarker.ReadMarkerEntity
		latestRM      *readmarker.ReadMarker
		pendNumBlocks int64
	)
	rme, err = b.packageHandler.GetLatestReadMarkerEntity(ctx, clientID)
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

		var response = &blobbergrpc.DownloadFileResponse{
			Success:      false,
			LatestRm:     convert.ReadMarkerToReadMarkerGRPC(latestRM),
			Path:         fileref.Path,
			AllocationId: fileref.AllocationID,
		}
		return response, nil
	}

	// check out read pool tokens if read_price > 0
	err = b.storageHandler.readPreRedeem(ctx, allocationObj, numBlocks, pendNumBlocks,
		clientIDForReadRedeem)
	if err != nil {
		return nil, common.NewErrorf("download_file",
			"pre-redeeming read marker: %v", err)
	}
	// reading allowed

	var (
		downloadMode = r.Content
		respData     []byte
	)
	if len(downloadMode) > 0 && downloadMode == DOWNLOAD_CONTENT_THUMB {
		var fileData = &filestore.FileInputData{}
		fileData.Name = fileref.Name
		fileData.Path = fileref.Path
		fileData.Hash = fileref.ThumbnailHash
		fileData.OnCloud = fileref.OnCloud
		respData, err = b.packageHandler.GetFileStore().GetFileBlock(allocationID,
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
		respData, err = b.packageHandler.GetFileStore().GetFileBlock(allocationID,
			fileData, blockNum, numBlocks)
		if err != nil {
			return nil, common.NewErrorf("download_file",
				"couldn't get file block: %v", err)
		}
	}

	readMarker.PayerID = clientIDForReadRedeem
	err = b.packageHandler.SaveLatestReadMarker(ctx, readMarker, latestRM == nil)
	if err != nil {
		return nil, common.NewErrorf("download_file",
			"couldn't save latest read marker: %v", err)
	}

	var response = &blobbergrpc.DownloadFileResponse{}
	response.Success = true
	response.LatestRm = convert.ReadMarkerToReadMarkerGRPC(readMarker)
	response.Data = respData
	response.Path = fileref.Path
	response.AllocationId = fileref.AllocationID

	b.packageHandler.FileBlockDownloaded(ctx, fileref.ID)
	//originally here is respData, need to clarify
	return response, nil
}
