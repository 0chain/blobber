package reference

import (
	"context"
	"path/filepath"

	"0chain.net/blobbercore/datastore"
	"0chain.net/core/common"
)

func GetReferencePath(ctx context.Context, allocationID string, path string) (*Ref, error) {
	return GetReferencePathFromPaths(ctx, allocationID, []string{path})
}

func GetReferencePathFromPaths(ctx context.Context, allocationID string, paths []string) (*Ref, error) {
	var refs []Ref
	db := datastore.GetStore().GetTransaction(ctx)
	pathsAdded := make(map[string]bool)
	for _, path := range paths {
		if _, ok := pathsAdded[path]; !ok {
			db = db.Where(Ref{ParentPath: path, AllocationID: allocationID})
			pathsAdded[path] = true
		}
		depth := len(GetSubDirsFromPath(path)) + 1
		curPath := filepath.Dir(path)
		for i := 0; i < depth-1; i++ {
			if _, ok := pathsAdded[curPath]; !ok {
				db = db.Or(Ref{ParentPath: curPath, AllocationID: allocationID})
				pathsAdded[curPath] = true
			}
			curPath = filepath.Dir(curPath)
		}
	}

	db = db.Or("parent_path = ? AND allocation_id = ?", "", allocationID)
	err := db.Order("level, lookup_hash").Find(&refs).Error
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return &Ref{Type: DIRECTORY, AllocationID: allocationID, Name: "/", Path: "/", ParentPath: "", PathLevel: 1}, nil
	}

	rootRef := &refs[0]
	if rootRef.Path != "/" {
		return nil, common.NewError("invalid_dir_tree", "DB has invalid tree. Root not found in DB")
	}

	refMap := make(map[string]*Ref)
	refMap[rootRef.Path] = rootRef
	for i := 1; i < len(refs); i++ {
		if _, ok := refMap[refs[i].ParentPath]; !ok {
			return nil, common.NewError("invalid_dir_tree", "DB has invalid tree.")
		}
		if _, ok := refMap[refs[i].Path]; !ok {
			refMap[refs[i].ParentPath].AddChild(&refs[i])
			refMap[refs[i].Path] = &refs[i]
		}
	}

	// curLevel := 2
	// subDirs := GetSubDirsFromPath(path)
	// var foundRef *Ref
	// for i := 1; i < len(refs); i++ {
	// 	if refs[i].ParentPath != curRef.Path && foundRef != nil {
	// 		curLevel++
	// 		curRef = foundRef
	// 		foundRef = nil
	// 	}

	// 	if refs[i].ParentPath == curRef.Path {
	// 		if len(subDirs) > (curLevel-2) && subDirs[curLevel-2] == refs[i].Name {
	// 			//curRef = &refs[i]
	// 			foundRef = &refs[i]
	// 		}
	// 		curRef.AddChild(&refs[i])
	// 	} else {
	// 		return nil, common.NewError("invalid_dir_tree", "DB has invalid tree.")
	// 	}
	// }
	refs[0].CalculateHash(ctx, false)
	return &refs[0], nil
}

func GetObjectTree(ctx context.Context, allocationID string, path string) (*Ref, error) {
	path = filepath.Clean(path)
	var refs []Ref
	db := datastore.GetStore().GetTransaction(ctx)
	db = db.Where(Ref{Path: path, AllocationID: allocationID})
	if path != "/" {
		db = db.Or("path LIKE ? AND allocation_id = ?", (path + "/%"), allocationID)
	} else {
		db = db.Or("path LIKE ? AND allocation_id = ?", (path + "%"), allocationID)
	}

	err := db.Order("level, lookup_hash").Find(&refs).Error
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid path. Could not find object tree")
	}
	childMap := make(map[string]*Ref)
	childMap[refs[0].Path] = &refs[0]
	for i := 1; i < len(refs); i++ {
		if _, ok := childMap[refs[i].ParentPath]; !ok {
			return nil, common.NewError("invalid_object_tree", "Invalid object tree")
		}
		childMap[refs[i].ParentPath].AddChild(&refs[i])
		childMap[refs[i].Path] = &refs[i]
	}
	return &refs[0], nil
}
