package convert

import (
	"bytes"
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
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
			ID:           t.ID,
			BlobberID:    t.BlobberID,
			AllocationID: t.AllocationID,
			ReadPrice:    t.ReadPrice,
			WritePrice:   t.WritePrice,
		})
	}
	return &blobbergrpc.Allocation{
		ID:               alloc.ID,
		Tx:               alloc.Tx,
		TotalSize:        alloc.TotalSize,
		UsedSize:         alloc.UsedSize,
		OwnerID:          alloc.OwnerID,
		OwnerPublicKey:   alloc.OwnerPublicKey,
		Expiration:       int64(alloc.Expiration),
		AllocationRoot:   alloc.AllocationRoot,
		BlobberSize:      alloc.BlobberSize,
		BlobberSizeUsed:  alloc.BlobberSizeUsed,
		LatestRedeemedWM: alloc.LatestRedeemedWM,
		IsRedeemRequired: alloc.IsRedeemRequired,
		TimeUnit:         int64(alloc.TimeUnit),
		CleanedUp:        alloc.CleanedUp,
		Finalized:        alloc.Finalized,
		Terms:            terms,
		PayerID:          alloc.PayerID,
	}
}

func GRPCAllocationToAllocation(alloc *blobbergrpc.Allocation) *allocation.Allocation {
	if alloc == nil {
		return nil
	}

	terms := make([]*allocation.Terms, 0, len(alloc.Terms))
	for _, t := range alloc.Terms {
		terms = append(terms, &allocation.Terms{
			ID:           t.ID,
			BlobberID:    t.BlobberID,
			AllocationID: t.AllocationID,
			ReadPrice:    t.ReadPrice,
			WritePrice:   t.WritePrice,
		})
	}
	return &allocation.Allocation{
		ID:               alloc.ID,
		Tx:               alloc.Tx,
		TotalSize:        alloc.TotalSize,
		UsedSize:         alloc.UsedSize,
		OwnerID:          alloc.OwnerID,
		OwnerPublicKey:   alloc.OwnerPublicKey,
		Expiration:       common.Timestamp(alloc.Expiration),
		AllocationRoot:   alloc.AllocationRoot,
		BlobberSize:      alloc.BlobberSize,
		BlobberSizeUsed:  alloc.BlobberSizeUsed,
		LatestRedeemedWM: alloc.LatestRedeemedWM,
		IsRedeemRequired: alloc.IsRedeemRequired,
		TimeUnit:         time.Duration(alloc.TimeUnit),
		CleanedUp:        alloc.CleanedUp,
		Finalized:        alloc.Finalized,
		Terms:            terms,
		PayerID:          alloc.PayerID,
	}
}

func FileStatsToFileStatsGRPC(fileStats *stats.FileStats) *blobbergrpc.FileStats {
	if fileStats == nil {
		return nil
	}

	return &blobbergrpc.FileStats{
		ID:                       fileStats.ID,
		RefID:                    fileStats.RefID,
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
		AllocationID:           wm.AllocationID,
		Size:                   wm.Size,
		BlobberID:              wm.BlobberID,
		Timestamp:              int64(wm.Timestamp),
		ClientID:               wm.ClientID,
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
		AllocationID:           wm.AllocationID,
		Size:                   wm.Size,
		BlobberID:              wm.BlobberID,
		Timestamp:              common.Timestamp(wm.Timestamp),
		ClientID:               wm.ClientID,
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
		ID:                       fileStats.ID,
		RefID:                    fileStats.RefID,
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

func DownloadFileGRPCToHTTP(req *blobbergrpc.DownloadFileRequest) (*http.Request, error) {
	body := bytes.NewBuffer([]byte{})
	writer := multipart.NewWriter(body)

	err := writer.WriteField("path", req.Path)
	if err != nil {
		return nil, err
	}

	err = writer.WriteField("path_hash", req.PathHash)
	if err != nil {
		return nil, err
	}

	err = writer.WriteField("rx_pay", req.RxPay)
	if err != nil {
		return nil, err
	}

	err = writer.WriteField("block_num", req.BlockNum)
	if err != nil {
		return nil, err
	}

	err = writer.WriteField("num_blocks", req.NumBlocks)
	if err != nil {
		return nil, err
	}

	err = writer.WriteField("read_marker", req.ReadMarker)
	if err != nil {
		return nil, err
	}

	err = writer.WriteField("auth_token", req.AuthToken)
	if err != nil {
		return nil, err
	}

	err = writer.WriteField("content", req.Content)
	if err != nil {
		return nil, err
	}

	r, err := http.NewRequest("POST", "", strings.NewReader(body.String()))
	if err != nil {
		return nil, err
	}

	r.Header.Set("Content-Type", writer.FormDataContentType())

	return r, nil
}
