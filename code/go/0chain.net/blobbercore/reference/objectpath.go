package reference

import (
	"context"
	"fmt"

	"0chain.net/core/common"
)

type ObjectPath struct {
	RootHash     string                 `json:"root_hash"`
	Meta         map[string]interface{} `json:"meta_data"`
	Path         map[string]interface{} `json:"path"`
	FileBlockNum int64                  `json:"file_block_num"`
	RefID        int64                  `json:"-"`
}

func GetObjectPath(ctx context.Context, allocationID string, blockNum int64) (*ObjectPath, error) {

	rootRef, err := GetRefWithSortedChildren(ctx, allocationID, "/")
	//fmt.Println("Root ref found with hash : " + rootRef.Hash)
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
			//result.Entities[idx] = child.GetListingData(ctx)

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

	//rootRef.CalculateHash(ctx, false)
	return &retObj, nil
}
