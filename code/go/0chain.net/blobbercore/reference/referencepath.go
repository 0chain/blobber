package reference

import (
	"context"
	"math"
	"path/filepath"
	"strings"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"gorm.io/gorm"
)

type ReferencePath struct {
	Meta map[string]interface{} `json:"meta_data"`
	List []*ReferencePath       `json:"list,omitempty"`
	Ref  *Ref
}

func GetReferencePath(ctx context.Context, allocationID, path string) (*Ref, error) {
	return GetReferenceForHashCalculationFromPaths(ctx, allocationID, []string{path})
}

// GetReferenceForHashCalculationFromPaths validate and build full dir tree from db, and CalculateHash and return root Ref without saving in DB
func GetReferenceForHashCalculationFromPaths(ctx context.Context, allocationID string, paths []string) (*Ref, error) {
	var refs []Ref
	db := datastore.GetStore().GetTransaction(ctx)
	db = db.Select("id", "allocation_id", "type", "name", "path",
		"parent_path", "size", "hash", "path_hash", "content_hash", "merkle_root",
		"actual_file_size", "actual_file_hash", "attributes", "chunk_size",
		"lookup_hash", "thumbnail_hash", "write_marker", "level")
	db = db.Model(&Ref{})
	pathsAdded := make(map[string]bool)
	for _, path := range paths {
		path = strings.TrimSuffix(path, "/")
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
	// root reference_objects with parent_path=""
	db = db.Or("parent_path = ? AND allocation_id = ?", "", allocationID)
	err := db.Order("path").Find(&refs).Error
	if err != nil {
		return nil, err
	}
	// there is no any child reference_objects for affected path, and insert root reference_objects
	if len(refs) == 0 {
		return &Ref{Type: DIRECTORY, AllocationID: allocationID, Name: "/", Path: "/", ParentPath: "", PathLevel: 1}, nil
	}
	rootRef := &refs[0]
	if rootRef.Path != "/" {
		return nil, common.NewError("invalid_dir_tree", "DB has invalid tree. Root not found in DB")
	}

	// valdiate dir tree, and populate Ref's children for CalculateHash
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

	return &refs[0], nil
}

// GetReferencePathFromPaths validate and build full dir tree from db, and CalculateHash and return root Ref
func GetReferencePathFromPaths(ctx context.Context, allocationID string, paths []string) (*Ref, error) {
	var refs []Ref
	db := datastore.GetStore().GetTransaction(ctx)
	pathsAdded := make(map[string]bool)
	for _, path := range paths {
		path = strings.TrimSuffix(path, "/")
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

	// root reference_objects with parent_path=""
	db = db.Or("parent_path = ? AND allocation_id = ?", "", allocationID)
	err := db.Order("path").Find(&refs).Error
	if err != nil {
		return nil, err
	}
	// there is no any child reference_objects for affected path, and insert root reference_objects
	if len(refs) == 0 {
		return &Ref{Type: DIRECTORY, AllocationID: allocationID, Name: "/", Path: "/", ParentPath: "", PathLevel: 1}, nil
	}

	rootRef := &refs[0]
	if rootRef.Path != "/" {
		return nil, common.NewError("invalid_dir_tree", "DB has invalid tree. Root not found in DB")
	}

	// valdiate dir tree, and populate Ref's children for CalculateHash
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

	return &refs[0], nil
}

func GetObjectTree(ctx context.Context, allocationID, path string) (*Ref, error) {
	path = filepath.Clean(path)
	var refs []Ref
	db := datastore.GetStore().GetTransaction(ctx)
	db = db.Where(Ref{Path: path, AllocationID: allocationID})
	if path != "/" {
		db = db.Or("path LIKE ? AND allocation_id = ?", path+"/%", allocationID)
	} else {
		db = db.Or("path LIKE ? AND allocation_id = ?", path+"%", allocationID)
	}
	err := db.Order("path").Find(&refs).Error
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

//This function retrieves reference_objects tables rows with pagination. Check for issue https://github.com/0chain/gosdk/issues/117
//Might need to consider covering index for efficient search https://blog.crunchydata.com/blog/why-covering-indexes-are-incredibly-helpful
//To retrieve refs efficiently form pagination index is created in postgresql on path column so it can be used to paginate refs
//very easily and effectively; Same case for offsetDate.
func GetRefs(ctx context.Context, allocationID, path, offsetPath, _type string, level, pageLimit int) (refs *[]PaginatedRef, totalPages int, newOffsetPath string, err error) {
	var totalRows int64
	var pRefs []PaginatedRef
	path = filepath.Clean(path)

	db := datastore.GetStore().GetDB()
	db1 := db.Session(&gorm.Session{})
	db2 := db.Session(&gorm.Session{})

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		db1 = db1.Model(&Ref{}).Where("allocation_id = ?", allocationID).
			Where(db1.Where("path = ?", path).Or("path LIKE ?", path+"%"))
		if _type != "" {
			db1 = db1.Where("type = ?", _type)
		}
		if level != 0 {
			db1 = db1.Where("level = ?", level)
		}

		db1 = db1.Where("path > ?", offsetPath)

		db1 = db1.Order("path")
		err = db1.Limit(pageLimit).Find(&pRefs).Error
		wg.Done()
	}()

	go func() {
		db2 = db2.Model(&Ref{}).Where("allocation_id = ?", allocationID).
			Where(db2.Where("path = ?", path).Or("path LIKE ?", path+"%"))
		if _type != "" {
			db2 = db2.Where("type = ?", _type)
		}
		if level != 0 {
			db2 = db2.Where("level = ?", level)
		}
		db2.Count(&totalRows)
		wg.Done()
	}()
	wg.Wait()
	if err != nil {
		return
	}

	refs = &pRefs
	if len(pRefs) > 0 {
		newOffsetPath = pRefs[len(pRefs)-1].Path
	}
	totalPages = int(math.Ceil(float64(totalRows) / float64(pageLimit)))
	return
}

//Retrieves updated refs compared to some update_at value. Useful to localCache
func GetUpdatedRefs(ctx context.Context, allocationID, path, offsetPath, _type, updatedDate, offsetDate string, level, pageLimit int, dateLayOut string) (refs *[]PaginatedRef, totalPages int, newOffsetPath, newOffsetDate string, err error) {
	var totalRows int64
	var pRefs []PaginatedRef
	db := datastore.GetStore().GetDB()
	db1 := db.Session(&gorm.Session{}) //TODO Might need to use transaction from db1/db2 to avoid injection attack
	db2 := db.Session(&gorm.Session{})

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		db1 = db1.Model(&Ref{}).Where("allocation_id = ?", allocationID).
			Where(db1.Where("path = ?", path).Or("path LIKE ?", path+"%"))
		if _type != "" {
			db1 = db1.Where("type = ?", _type)
		}
		if level != 0 {
			db1 = db1.Where("level = ?", level)
		}
		if updatedDate != "" {
			db1 = db1.Where("updated_at > ?", updatedDate)
		}

		if offsetDate != "" {
			db1 = db1.Where("(updated_at, path) > (?, ?)", offsetDate, offsetPath)
		}
		db1 = db1.Order("updated_at, path")
		db1 = db1.Limit(pageLimit)
		err = db1.Find(&pRefs).Error
		wg.Done()
	}()

	go func() {
		db2 = db2.Model(&Ref{}).Where("allocation_id = ?", allocationID).
			Where(db2.Where("path = ?", path).Or("path LIKE ?", path+"%"))
		if _type != "" {
			db2 = db2.Where("type > ?", level)
		}
		if level != 0 {
			db2 = db2.Where("level = ?", level)
		}
		if updatedDate != "" {
			db2 = db2.Where("updated_at > ?", updatedDate)
		}
		db2 = db2.Count(&totalRows)
		wg.Done()
	}()
	wg.Wait()
	if err != nil {
		return
	}

	if len(pRefs) != 0 {
		lastIdx := len(pRefs) - 1
		newOffsetDate = pRefs[lastIdx].UpdatedAt.Format(dateLayOut)
		newOffsetPath = pRefs[lastIdx].Path
	}
	refs = &pRefs
	totalPages = int(math.Ceil(float64(totalRows) / float64(pageLimit)))
	return
}

//Retrieves deleted refs compared to some update_at value. Useful for localCache.
func GetDeletedRefs(ctx context.Context, allocationID, updatedDate, offsetPath, offsetDate string, pageLimit int, dateLayOut string) (refs *[]PaginatedRef, totalPages int, newOffsetPath, newOffsetDate string, err error) {
	var totalRows int64
	var pRefs []PaginatedRef
	db := datastore.GetStore().GetDB()

	db1 := db.Session(&gorm.Session{})
	db2 := db.Session(&gorm.Session{})

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		db1 = db1.Model(&Ref{}).Unscoped().
			Select("path", "path_hash", "deleted_at", "updated_at").
			Where("allocation_id = ?", allocationID)

		if updatedDate == "" {
			db1 = db1.Where("deleted_at IS NOT null")
		} else {
			db1 = db1.Where("deleted_at > ?", updatedDate)
		}

		if offsetDate != "" {
			db1 = db1.Where("(updated_at, path) > (?, ?)", offsetDate, offsetPath)
		}

		err = db1.Order("updated_at, path").Limit(pageLimit).Find(&pRefs).Error
		wg.Done()
	}()

	go func() {
		db2 = db2.Model(&Ref{}).Unscoped().Where("allocation_id = ?", allocationID)

		if updatedDate == "" {
			db2 = db2.Where("deleted_at IS NOT null")
		} else {
			db2 = db2.Where("deleted_at > ?", updatedDate)
		}

		db2 = db2.Count(&totalRows)
		wg.Done()
	}()
	wg.Wait()
	if len(pRefs) != 0 {
		lastIdx := len(pRefs) - 1
		newOffsetDate = pRefs[lastIdx].DeletedAt.Time.Format(dateLayOut)
		newOffsetPath = pRefs[lastIdx].Path
	}
	refs = &pRefs
	totalPages = int(math.Ceil(float64(totalRows) / float64(pageLimit)))
	return
}
