package reference

import (
	"context"
	"strings"

	"0chain.net/common"
	"0chain.net/datastore"
)

type ReferencePath struct {
	Meta map[string]interface{} `json:"meta_data"`
	List []*ReferencePath       `json:"list,omitempty"`
}

func getSubDirs(p string) []string {
	subDirs := strings.Split(p, "/")
	tSubDirs := make([]string, 0)
	for _, s := range subDirs {
		if s != "" {
			tSubDirs = append(tSubDirs, s)
		}
	}
	return tSubDirs
}

func GetReferencePath(ctx context.Context, allocationID string, filePath string, dbStore datastore.Store) (*ReferencePath, error) {

	rootRef, err := GetRootReferenceFromStore(ctx, allocationID, dbStore)

	//fmt.Println("Root ref found with hash : " + rootRef.Hash)
	if err != nil {
		return nil, common.NewError("invalid_dir_struct", "Allocation root corresponds to an invalid directory structure")
	}

	if len(filePath) == 0 {
		return nil, common.NewError("invalid_path", "Invalid filepath")
	}

	//path, _ := filepath.Split(filePath)
	subDirs := getSubDirs(filePath)

	result := &ReferencePath{}
	curRef := rootRef
	curResult := result

	for _, subDir := range subDirs {
		found := false
		curResult.Meta = curRef.GetListingData(ctx)
		err := curRef.LoadChildren(ctx, dbStore)
		if err != nil {
			return nil, common.NewError("error_loading_children", "Error loading children from store for path "+curRef.Path)
		}
		list := make([]*ReferencePath, len(curRef.Children))
		curResult.List = list
		var foundRef *Ref
		foundIdx := -1
		for idx, child := range curRef.Children {
			list[idx] = &ReferencePath{}
			list[idx].Meta = child.GetListingData(ctx)
			if subDir == child.GetName() && child.GetType() == DIRECTORY {
				foundRef = child.(*Ref)
				foundIdx = idx
				found = true
			}
		}
		if !found {
			break
		}
		curRef = foundRef
		curResult = list[foundIdx]
	}

	curResult.Meta = curRef.GetListingData(ctx)
	err = curRef.LoadChildren(ctx, dbStore)
	if err != nil {
		return nil, common.NewError("error_loading_children", "Error loading children from store for path "+curRef.Path)
	}
	list := make([]*ReferencePath, len(curRef.Children))
	curResult.List = list
	for idx, child := range curRef.Children {
		list[idx] = &ReferencePath{}
		list[idx].Meta = child.GetListingData(ctx)
	}
	return result, nil
}
