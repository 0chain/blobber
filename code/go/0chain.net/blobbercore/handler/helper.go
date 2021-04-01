package handler

import (
	"context"

	"0chain.net/blobbercore/reference"
	"0chain.net/blobbercore/stats"

	"0chain.net/blobbercore/allocation"
	"0chain.net/blobbercore/blobbergrpc"
	"0chain.net/blobbercore/constants"
)

func setupGRPCHandlerContext(ctx context.Context, r *blobbergrpc.RequestContext) context.Context {
	ctx = context.WithValue(ctx, constants.CLIENT_CONTEXT_KEY,
		r.Client)
	ctx = context.WithValue(ctx, constants.CLIENT_KEY_CONTEXT_KEY,
		r.ClientKey)
	ctx = context.WithValue(ctx, constants.ALLOCATION_CONTEXT_KEY,
		r.Allocation)
	return ctx
}

func convertAllocationToGRPCAllocation(alloc *allocation.Allocation) *blobbergrpc.Allocation {
	terms := make([]*blobbergrpc.Term, len(alloc.Terms))
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

func convertFileStatsToFileStatsGRPC(fileStats *stats.FileStats) *blobbergrpc.FileStats {
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
