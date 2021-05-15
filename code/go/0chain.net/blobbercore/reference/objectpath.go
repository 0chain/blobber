package reference

import (
	"context"
	"fmt"
	"time"

	"0chain.net/blobbercore/blobbergrpc"

	"0chain.net/core/common"
)

type ObjectPath struct {
	RootHash     string                 `json:"root_hash"`
	Meta         map[string]interface{} `json:"meta_data"`
	Path         map[string]interface{} `json:"path"`
	FileBlockNum int64                  `json:"file_block_num"`
	RefID        int64                  `json:"-"`
}

// TODO needs to be refactored, current implementation can probably be heavily simplified
func GetObjectPath(ctx context.Context, allocationID string, blockNum int64) (*ObjectPath, error) {

	rootRef, err := GetRefWithSortedChildren(ctx, allocationID, "/")
	if err != nil {
		return nil, common.NewError("invalid_dir_struct", "Allocation root corresponds to an invalid directory structure")
	}

	if rootRef.NumBlocks < blockNum {
		return nil, common.NewError("invalid_block_num", fmt.Sprintf("Invalid block number %d/%d", rootRef.NumBlocks, blockNum))
	}

	if rootRef.NumBlocks == 0 {
		var retObj ObjectPath
		retObj.RootHash = rootRef.Hash
		retObj.FileBlockNum = 0
		result := rootRef.GetListingData(ctx)
		list := make([]map[string]interface{}, len(rootRef.Children))
		for idx, child := range rootRef.Children {
			list[idx] = child.GetListingData(ctx)
		}
		result["list"] = list
		retObj.Path = result
		return &retObj, nil
	}

	found := false
	var curRef *Ref
	curRef = rootRef
	remainingBlocks := blockNum

	result := curRef.GetListingData(ctx)
	curResult := result

	for !found {
		list := make([]map[string]interface{}, len(curRef.Children))
		for idx, child := range curRef.Children {
			list[idx] = child.GetListingData(ctx)
		}
		curResult["list"] = list
		for idx, child := range curRef.Children {

			if child.NumBlocks < remainingBlocks {
				remainingBlocks = remainingBlocks - child.NumBlocks
				continue
			}
			if child.Type == FILE {
				found = true
				curRef = child
				break
			}
			curRef, err = GetRefWithSortedChildren(ctx, allocationID, child.Path)
			if err != nil || len(curRef.Hash) == 0 {
				return nil, common.NewError("failed_object_path", "Failed to get the object path")
			}
			curResult = list[idx]
			break
		}
	}
	if !found {
		return nil, common.NewError("invalid_parameters", "Block num was not found")
	}

	var retObj ObjectPath
	retObj.RootHash = rootRef.Hash
	retObj.Meta = curRef.GetListingData(ctx)
	retObj.Path = result
	retObj.FileBlockNum = remainingBlocks
	retObj.RefID = curRef.ID

	return &retObj, nil
}

// TODO needs to be refactored, current implementation can probably be heavily simplified
func GetObjectPathGRPC(ctx context.Context, allocationID string, blockNum int64) (*blobbergrpc.ObjectPath, error) {

	rootRef, err := GetRefWithSortedChildren(ctx, allocationID, "/")
	if err != nil {
		return nil, common.NewError("invalid_dir_struct", "Allocation root corresponds to an invalid directory structure")
	}

	if rootRef.NumBlocks < blockNum {
		return nil, common.NewError("invalid_block_num", fmt.Sprintf("Invalid block number %d/%d", rootRef.NumBlocks, blockNum))
	}

	if rootRef.NumBlocks == 0 {
		children := make([]*blobbergrpc.FileRef, len(rootRef.Children))
		for idx, child := range rootRef.Children {
			children[idx] = FileRefToFileRefGRPC(child)
		}
		path := FileRefToFileRefGRPC(rootRef)
		path.DirMetaData.Children = children
		return &blobbergrpc.ObjectPath{
			RootHash:     rootRef.Hash,
			Path:         path,
			FileBlockNum: 0,
		}, nil
	}

	found := false
	var curRef *Ref
	curRef = rootRef
	remainingBlocks := blockNum

	result := curRef.GetListingData(ctx)
	curResult := result

	for !found {
		list := make([]map[string]interface{}, len(curRef.Children))
		for idx, child := range curRef.Children {
			list[idx] = child.GetListingData(ctx)
		}
		curResult["list"] = list
		for idx, child := range curRef.Children {

			if child.NumBlocks < remainingBlocks {
				remainingBlocks = remainingBlocks - child.NumBlocks
				continue
			}
			if child.Type == FILE {
				found = true
				curRef = child
				break
			}
			curRef, err = GetRefWithSortedChildren(ctx, allocationID, child.Path)
			if err != nil || len(curRef.Hash) == 0 {
				return nil, common.NewError("failed_object_path", "Failed to get the object path")
			}
			curResult = list[idx]
			break
		}
	}
	if !found {
		return nil, common.NewError("invalid_parameters", "Block num was not found")
	}

	var children []*blobbergrpc.FileRef
	for _, child := range rootRef.Children {
		children = append(children, FileRefToFileRefGRPC(child))
	}
	path := FileRefToFileRefGRPC(rootRef)
	path.DirMetaData.Children = children
	return &blobbergrpc.ObjectPath{
		RootHash:     rootRef.Hash,
		Meta:         FileRefToFileRefGRPC(curRef),
		Path:         path,
		FileBlockNum: remainingBlocks,
	}, nil
}

func FileRefToFileRefGRPC(ref *Ref) *blobbergrpc.FileRef {

	var fileMetaData *blobbergrpc.FileMetaData
	var dirMetaData *blobbergrpc.DirMetaData
	switch ref.Type {
	case FILE:
		fileMetaData = convertFileRefToFileMetaDataGRPC(ref)
	case DIRECTORY:
		dirMetaData = convertDirRefToDirMetaDataGRPC(ref)
	}

	return &blobbergrpc.FileRef{
		Type:         ref.Type,
		FileMetaData: fileMetaData,
		DirMetaData:  dirMetaData,
	}
}

func convertFileRefToFileMetaDataGRPC(fileref *Ref) *blobbergrpc.FileMetaData {
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

func convertDirRefToDirMetaDataGRPC(dirref *Ref) *blobbergrpc.DirMetaData {
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

func FileRefGRPCToFileRef(ref *blobbergrpc.FileRef) *Ref {
	switch ref.Type {
	case FILE:
		return convertFileMetaDataGRPCToFileRef(ref.FileMetaData)
	case DIRECTORY:
		return convertDirMetaDataGRPCToDirRef(ref.DirMetaData)
	}

	return nil
}

func convertFileMetaDataGRPCToFileRef(metaData *blobbergrpc.FileMetaData) *Ref {
	var commitMetaTxnsGRPC []CommitMetaTxn
	for _, c := range metaData.CommitMetaTxns {
		commitMetaTxnsGRPC = append(commitMetaTxnsGRPC, CommitMetaTxn{
			RefID:     c.RefId,
			TxnID:     c.TxnId,
			CreatedAt: time.Unix(0, c.CreatedAt),
		})
	}
	return &Ref{
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

func convertDirMetaDataGRPCToDirRef(dirref *blobbergrpc.DirMetaData) *Ref {
	return &Ref{
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
