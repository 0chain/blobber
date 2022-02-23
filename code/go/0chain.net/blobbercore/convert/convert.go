package convert

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

func AllocationToGRPCAllocation(alloc *allocation.Allocation) *blobbergrpc.Allocation {
	if alloc == nil {
		return nil
	}

	terms := make([]*blobbergrpc.Term, 0, len(alloc.Terms))
	for _, t := range alloc.Terms {
		terms = append(terms, &blobbergrpc.Term{
			Id:           t.ID,
			BlobberId:    t.BlobberID,
			AllocationId: t.AllocationID,
			ReadPrice:    t.ReadPrice,
			WritePrice:   t.WritePrice,
		})
	}
	return &blobbergrpc.Allocation{
		Id:               alloc.ID,
		Tx:               alloc.Tx,
		TotalSize:        alloc.TotalSize,
		UsedSize:         alloc.UsedSize,
		OwnerId:          alloc.OwnerID,
		OwnerPublicKey:   alloc.OwnerPublicKey,
		Expiration:       int64(alloc.Expiration),
		AllocationRoot:   alloc.AllocationRoot,
		BlobberSize:      alloc.BlobberSize,
		BlobberSizeUsed:  alloc.BlobberSizeUsed,
		LatestRedeemedWm: alloc.LatestRedeemedWM,
		IsRedeemRequired: alloc.IsRedeemRequired,
		TimeUnit:         int64(alloc.TimeUnit),
		CleanedUp:        alloc.CleanedUp,
		Finalized:        alloc.Finalized,
		Terms:            terms,
		PayerId:          alloc.PayerID,
	}
}

func GRPCAllocationToAllocation(alloc *blobbergrpc.Allocation) *allocation.Allocation {
	if alloc == nil {
		return nil
	}

	terms := make([]*allocation.Terms, 0, len(alloc.Terms))
	for _, t := range alloc.Terms {
		terms = append(terms, &allocation.Terms{
			ID:           t.Id,
			BlobberID:    t.BlobberId,
			AllocationID: t.AllocationId,
			ReadPrice:    t.ReadPrice,
			WritePrice:   t.WritePrice,
		})
	}
	return &allocation.Allocation{
		ID:               alloc.Id,
		Tx:               alloc.Tx,
		TotalSize:        alloc.TotalSize,
		UsedSize:         alloc.UsedSize,
		OwnerID:          alloc.OwnerId,
		OwnerPublicKey:   alloc.OwnerPublicKey,
		Expiration:       common.Timestamp(alloc.Expiration),
		AllocationRoot:   alloc.AllocationRoot,
		BlobberSize:      alloc.BlobberSize,
		BlobberSizeUsed:  alloc.BlobberSizeUsed,
		LatestRedeemedWM: alloc.LatestRedeemedWm,
		IsRedeemRequired: alloc.IsRedeemRequired,
		TimeUnit:         time.Duration(alloc.TimeUnit),
		CleanedUp:        alloc.CleanedUp,
		Finalized:        alloc.Finalized,
		Terms:            terms,
		PayerID:          alloc.PayerId,
	}
}

func FileStatsToFileStatsGRPC(fileStats *stats.FileStats) *blobbergrpc.FileStats {
	if fileStats == nil {
		return nil
	}

	return &blobbergrpc.FileStats{
		Id:                       fileStats.ID,
		RefId:                    fileStats.RefID,
		NumUpdates:               fileStats.NumUpdates,
		NumBlockDownloads:        fileStats.NumBlockDownloads,
		SuccessChallenges:        fileStats.SuccessChallenges,
		FailedChallenges:         fileStats.FailedChallenges,
		LastChallengeResponseTxn: fileStats.LastChallengeResponseTxn,
		WriteMarkerRedeemTxn:     fileStats.WriteMarkerRedeemTxn,
		CreatedAt:                fileStats.CreatedAt.UnixNano(),
		UpdatedAt:                fileStats.UpdatedAt.UnixNano(),
	}
}

func WriteMarkerToWriteMarkerGRPC(wm *writemarker.WriteMarker) *blobbergrpc.WriteMarker {
	if wm == nil {
		return nil
	}

	return &blobbergrpc.WriteMarker{
		AllocationRoot:         wm.AllocationRoot,
		PreviousAllocationRoot: wm.PreviousAllocationRoot,
		AllocationId:           wm.AllocationID,
		Size:                   wm.Size,
		BlobberId:              wm.BlobberID,
		Timestamp:              int64(wm.Timestamp),
		ClientId:               wm.ClientID,
		Signature:              wm.Signature,
	}
}

func WriteMarkerGRPCToWriteMarker(wm *blobbergrpc.WriteMarker) *writemarker.WriteMarker {
	if wm == nil {
		return nil
	}

	return &writemarker.WriteMarker{
		AllocationRoot:         wm.AllocationRoot,
		PreviousAllocationRoot: wm.PreviousAllocationRoot,
		AllocationID:           wm.AllocationId,
		Size:                   wm.Size,
		BlobberID:              wm.BlobberId,
		Timestamp:              common.Timestamp(wm.Timestamp),
		ClientID:               wm.ClientId,
		Signature:              wm.Signature,
	}
}

func ReadMarkerToReadMarkerGRPC(rm *readmarker.ReadMarker) *blobbergrpc.ReadMaker {
	if rm == nil {
		return nil
	}

	return &blobbergrpc.ReadMaker{
		ClientId:        rm.ClientID,
		ClientPublicKey: rm.ClientPublicKey,
		BlobberId:       rm.BlobberID,
		AllocationId:    rm.AllocationID,
		OwnerId:         rm.OwnerID,
		Timestamp:       int64(rm.Timestamp),
		Counter:         rm.ReadCounter,
		Signature:       rm.Signature,
		Suspend:         rm.Suspend,
		PayerId:         rm.PayerID,
		AuthTicket:      rm.AuthTicket,
	}
}

func ReadMakerGRPCToReadMaker(rm *blobbergrpc.ReadMaker) *readmarker.ReadMarker {
	if rm == nil {
		return nil
	}

	return &readmarker.ReadMarker{
		ClientID:        rm.ClientId,
		ClientPublicKey: rm.ClientPublicKey,
		BlobberID:       rm.BlobberId,
		AllocationID:    rm.AllocationId,
		OwnerID:         rm.OwnerId,
		Timestamp:       common.Timestamp(rm.Timestamp),
		ReadCounter:     rm.Counter,
		Signature:       rm.Signature,
		Suspend:         rm.Suspend,
		PayerID:         rm.PayerId,
		AuthTicket:      rm.AuthTicket,
	}
}

func FileStatsGRPCToFileStats(fileStats *blobbergrpc.FileStats) *stats.FileStats {
	if fileStats == nil {
		return nil
	}

	return &stats.FileStats{
		ID:                       fileStats.Id,
		RefID:                    fileStats.RefId,
		NumUpdates:               fileStats.NumUpdates,
		NumBlockDownloads:        fileStats.NumBlockDownloads,
		SuccessChallenges:        fileStats.SuccessChallenges,
		FailedChallenges:         fileStats.FailedChallenges,
		LastChallengeResponseTxn: fileStats.LastChallengeResponseTxn,
		WriteMarkerRedeemTxn:     fileStats.WriteMarkerRedeemTxn,
		ModelWithTS: datastore.ModelWithTS{
			CreatedAt: time.Unix(0, fileStats.CreatedAt),
			UpdatedAt: time.Unix(0, fileStats.UpdatedAt),
		},
	}
}

func CollaboratorToGRPCCollaborator(c *reference.Collaborator) *blobbergrpc.Collaborator {
	if c == nil {
		return nil
	}

	return &blobbergrpc.Collaborator{
		RefId:     c.RefID,
		ClientId:  c.ClientID,
		CreatedAt: c.CreatedAt.UnixNano(),
	}
}

func GRPCCollaboratorToCollaborator(c *blobbergrpc.Collaborator) *reference.Collaborator {
	if c == nil {
		return nil
	}

	return &reference.Collaborator{
		RefID:     c.RefId,
		ClientID:  c.ClientId,
		CreatedAt: time.Unix(0, c.CreatedAt),
	}
}

func ReferencePathToReferencePathGRPC(recursionCount *int, refPath *reference.ReferencePath) *blobbergrpc.ReferencePath {
	if refPath == nil {
		return nil
	}
	// Accounting for bad reference paths where child path points to parent path and causes this algorithm to never end
	*recursionCount += 1
	defer func() {
		*recursionCount -= 1
	}()

	if *recursionCount > 150 {
		return &blobbergrpc.ReferencePath{
			MetaData: FileRefToFileRefGRPC(reference.ListingDataToRef(refPath.Meta)),
			List:     nil,
		}
	}

	var list []*blobbergrpc.ReferencePath
	for i := range refPath.List {
		list = append(list, ReferencePathToReferencePathGRPC(recursionCount, refPath.List[i]))
	}

	return &blobbergrpc.ReferencePath{
		MetaData: FileRefToFileRefGRPC(reference.ListingDataToRef(refPath.Meta)),
		List:     list,
	}
}

func ReferencePathGRPCToReferencePath(recursionCount *int, refPath *blobbergrpc.ReferencePath) *reference.ReferencePath {
	if refPath == nil {
		return nil
	}
	// Accounting for bad reference paths where child path points to parent path and causes this algorithm to never end
	*recursionCount += 1
	defer func() {
		*recursionCount -= 1
	}()

	if *recursionCount > 150 {
		return &reference.ReferencePath{
			Meta: FileRefGRPCToFileRef(refPath.MetaData).GetListingData(context.Background()),
			List: nil,
		}
	}

	var list []*reference.ReferencePath
	for i := range refPath.List {
		list = append(list, ReferencePathGRPCToReferencePath(recursionCount, refPath.List[i]))
	}

	return &reference.ReferencePath{
		Meta: FileRefGRPCToFileRef(refPath.MetaData).GetListingData(context.Background()),
		List: list,
	}
}

func FileRefToFileRefGRPC(ref *reference.Ref) *blobbergrpc.FileRef {
	if ref == nil {
		return nil
	}

	var fileMetaData *blobbergrpc.FileMetaData
	var dirMetaData *blobbergrpc.DirMetaData
	switch ref.Type {
	case reference.FILE:
		fileMetaData = convertFileRefToFileMetaDataGRPC(ref)
	case reference.DIRECTORY:
		dirMetaData = convertDirRefToDirMetaDataGRPC(ref)
	}

	return &blobbergrpc.FileRef{
		Type:         ref.Type,
		FileMetaData: fileMetaData,
		DirMetaData:  dirMetaData,
	}
}

func convertFileRefToFileMetaDataGRPC(fileref *reference.Ref) *blobbergrpc.FileMetaData {
	var commitMetaTxnsGRPC []*blobbergrpc.CommitMetaTxn
	for _, c := range fileref.CommitMetaTxns {
		commitMetaTxnsGRPC = append(commitMetaTxnsGRPC, &blobbergrpc.CommitMetaTxn{
			RefId:     c.RefID,
			TxnId:     c.TxnID,
			CreatedAt: c.CreatedAt.UnixNano(),
		})
	}
	return &blobbergrpc.FileMetaData{
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
	}
}

func convertDirRefToDirMetaDataGRPC(dirref *reference.Ref) *blobbergrpc.DirMetaData {
	return &blobbergrpc.DirMetaData{
		Type:       dirref.Type,
		LookupHash: dirref.LookupHash,
		Name:       dirref.Name,
		Path:       dirref.Path,
		Hash:       dirref.Hash,
		NumBlocks:  dirref.NumBlocks,
		PathHash:   dirref.PathHash,
		Size:       dirref.Size,
		CreatedAt:  dirref.CreatedAt.UnixNano(),
		UpdatedAt:  dirref.UpdatedAt.UnixNano(),
	}
}

func FileRefGRPCToFileRef(ref *blobbergrpc.FileRef) *reference.Ref {
	if ref == nil {
		return nil
	}

	switch ref.Type {
	case reference.FILE:
		return convertFileMetaDataGRPCToFileRef(ref.FileMetaData)
	case reference.DIRECTORY:
		return convertDirMetaDataGRPCToDirRef(ref.DirMetaData)
	}

	return nil
}

func convertFileMetaDataGRPCToFileRef(metaData *blobbergrpc.FileMetaData) *reference.Ref {
	var commitMetaTxnsGRPC []reference.CommitMetaTxn
	for _, c := range metaData.CommitMetaTxns {
		commitMetaTxnsGRPC = append(commitMetaTxnsGRPC, reference.CommitMetaTxn{
			RefID:     c.RefId,
			TxnID:     c.TxnId,
			CreatedAt: time.Unix(0, c.CreatedAt),
		})
	}
	return &reference.Ref{
		Type:                metaData.Type,
		LookupHash:          metaData.LookupHash,
		Name:                metaData.Name,
		Path:                metaData.Path,
		Hash:                metaData.Hash,
		NumBlocks:           metaData.NumBlocks,
		PathHash:            metaData.PathHash,
		CustomMeta:          metaData.CustomMeta,
		ContentHash:         metaData.ContentHash,
		Size:                metaData.Size,
		MerkleRoot:          metaData.MerkleRoot,
		ActualFileSize:      metaData.ActualFileSize,
		ActualFileHash:      metaData.ActualFileHash,
		MimeType:            metaData.MimeType,
		ThumbnailSize:       metaData.ThumbnailSize,
		ThumbnailHash:       metaData.ThumbnailHash,
		ActualThumbnailSize: metaData.ActualThumbnailSize,
		ActualThumbnailHash: metaData.ActualThumbnailHash,
		EncryptedKey:        metaData.EncryptedKey,
		Attributes:          metaData.Attributes,
		OnCloud:             metaData.OnCloud,
		CommitMetaTxns:      commitMetaTxnsGRPC,
		CreatedAt:           time.Unix(0, metaData.CreatedAt),
		UpdatedAt:           time.Unix(0, metaData.UpdatedAt),
	}
}

func convertDirMetaDataGRPCToDirRef(dirref *blobbergrpc.DirMetaData) *reference.Ref {
	return &reference.Ref{
		Type:       dirref.Type,
		LookupHash: dirref.LookupHash,
		Name:       dirref.Name,
		Path:       dirref.Path,
		Hash:       dirref.Hash,
		NumBlocks:  dirref.NumBlocks,
		PathHash:   dirref.PathHash,
		Size:       dirref.Size,
		CreatedAt:  time.Unix(0, dirref.CreatedAt),
		UpdatedAt:  time.Unix(0, dirref.UpdatedAt),
	}
}

func WriteFileGRPCToHTTP(req *blobbergrpc.UploadFileRequest) (*http.Request, error) {
	var formData allocation.UpdateFileChanger
	var uploadMetaString string
	switch req.Method {
	case `POST`:
		uploadMetaString = req.UploadMeta
	case `PUT`:
		uploadMetaString = req.UpdateMeta
	}
	err := json.Unmarshal([]byte(uploadMetaString), &formData)
	if err != nil {
		return nil, common.NewError("invalid_parameters",
			"Invalid parameters. Error parsing the meta data for upload."+err.Error())
	}

	r, err := http.NewRequest(req.Method, "", http.NoBody)
	if err != nil {
		return nil, err
	}

	if req.Method != `DELETE` {
		body := bytes.NewBuffer(nil)
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile(`uploadFile`, formData.Filename)
		if err != nil {
			return nil, err
		}
		_, err = part.Write(req.UploadFile)
		if err != nil {
			return nil, err
		}

		thumbPart, err := writer.CreateFormFile(`uploadThumbnailFile`, formData.ThumbnailFilename)
		if err != nil {
			return nil, err
		}
		_, err = thumbPart.Write(req.UploadThumbnailFile)
		if err != nil {
			return nil, err
		}

		err = writer.Close()
		if err != nil {
			return nil, err
		}

		r, err = http.NewRequest(req.Method, "", body)
		if err != nil {
			return nil, err
		}
		r.Header.Set("Content-Type", writer.FormDataContentType())
	}

	return r, nil
}

func DownloadFileGRPCToHTTP(req *blobbergrpc.DownloadFileRequest) (*http.Request, error) {

	r, err := http.NewRequest("GET", "", nil)
	if err != nil {
		return nil, err
	}

	r.Header.Set("path", req.Path)
	r.Header.Set("path_hash", req.PathHash)
	r.Header.Set("rx_pay", req.RxPay)
	r.Header.Set("block_num", req.BlockNum)
	r.Header.Set("num_blocks", req.NumBlocks)
	r.Header.Set("read_marker", req.ReadMarker)
	r.Header.Set("auth_token", req.AuthToken)
	r.Header.Set("content", req.Content)
	return r, nil
}
