package reference

import (
	"context"
	"math"
	"path/filepath"
	"strings"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ReferencePath struct {
	Meta map[string]interface{} `json:"meta_data"`
	List []*ReferencePath       `json:"list,omitempty"`
	Ref  *Ref                   `json:"-"`
}

func GetReferencePath(ctx context.Context, allocationID, path string) (*Ref, error) {
	return GetReferenceForHashCalculationFromPaths(ctx, allocationID, []string{path})
}

// GetReferenceForHashCalculationFromPaths validate and build full dir tree from db, and CalculateHash and return root Ref without saving in DB
func GetReferenceForHashCalculationFromPaths(ctx context.Context, allocationID string, paths []string) (*Ref, error) {
	var refs []Ref
	t := datastore.GetStore().GetTransaction(ctx)
	db := t.Model(&Ref{}).Select("id", "allocation_id", "type", "name", "path", "parent_path", "size", "hash", "file_meta_hash",
		"path_hash", "validation_root", "fixed_merkle_root", "actual_file_size", "actual_file_hash", "chunk_size",
		"lookup_hash", "thumbnail_hash", "allocation_root", "level", "created_at", "updated_at", "file_id")

	pathsAdded := make(map[string]bool)
	var shouldOr bool
	for _, path := range paths {
		if _, ok := pathsAdded[path]; !ok {
			if !shouldOr {
				db = db.Where("allocation_id=? AND parent_path=?", allocationID, path)
				shouldOr = true
			} else {
				db = db.Or(Ref{ParentPath: path, AllocationID: allocationID})
			}
			pathsAdded[path] = true
		}
		fields, err := common.GetPathFields(path)
		if err != nil {
			return nil, err
		}

		curPath := filepath.Dir(path)
		for i := 0; i <= len(fields); i++ {
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
			return nil, common.NewError("invalid_dir_tree", "DB has invalid tree."+
				"Path is: "+refs[i].ParentPath)
		}
		if _, ok := refMap[refs[i].Path]; !ok {
			refMap[refs[i].ParentPath].AddChild(&refs[i])
			refMap[refs[i].Path] = &refs[i]
		}
	}

	return &refs[0], nil
}

func (rootRef *Ref) GetSrcPath(path string) (*Ref, error) {

	path = filepath.Clean(path)

	if path == "/" {
		newRoot := *rootRef
		return &newRoot, nil
	}

	fields, err := common.GetPathFields(path)
	if err != nil {
		return nil, err
	}

	dirRef := rootRef
	for i := 0; i < len(fields); i++ {
		found := false
		for _, child := range dirRef.Children {
			if child.Name == fields[i] {
				dirRef = child
				found = true
			}
		}
		if !found {
			return nil, common.NewError("invalid_path", "ref is not found")
		}
	}
	newDirRef := *dirRef
	return &newDirRef, nil
}

// GetReferencePathFromPaths validate and build full dir tree from db, and CalculateHash and return root Ref
func GetReferencePathFromPaths(ctx context.Context, allocationID string, paths, objTreePath []string) (*Ref, error) {
	var refs []Ref
	t := datastore.GetStore().GetTransaction(ctx)
	db := t.DB

	pathsAdded := make(map[string]bool)
	var shouldOr bool
	for _, path := range paths {
		path = strings.TrimSuffix(path, "/")
		if _, ok := pathsAdded[path]; !ok {
			if shouldOr {
				db = db.Or(Ref{ParentPath: path, AllocationID: allocationID})
			} else {
				db = db.Where(Ref{ParentPath: path, AllocationID: allocationID})
				shouldOr = true
			}
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
		refMap[refs[i].ParentPath].AddChild(&refs[i])
		refMap[refs[i].Path] = &refs[i]

	}

	for _, path := range objTreePath {
		ref, err := GetObjectTree(ctx, allocationID, path)
		if err != nil {
			return nil, err
		}
		if _, ok := refMap[path]; !ok {
			_, found := refMap[ref.ParentPath]
			if !found {
				return nil, common.NewError("invalid_dir_tree", "DB has invalid tree Parent path not found for object tree.")
			}
			refMap[ref.ParentPath].AddChild(ref)
			refMap[ref.Path] = ref
		} else {
			refMap[ref.Path].Children = ref.Children
			refMap[ref.Path].childrenLoaded = true
		}
	}

	return &refs[0], nil
}

func GetObjectTree(ctx context.Context, allocationID, path string) (*Ref, error) {
	path = filepath.Clean(path)
	var refs []Ref
	t := datastore.GetStore().GetTransaction(ctx)
	db := t.Where(Ref{Path: path, AllocationID: allocationID})
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
		// root is not created if it is new empty allocation
		if path == "/" {
			return &Ref{Type: DIRECTORY, Path: "/", Name: "/", ParentPath: "", PathLevel: 1}, nil
		}

		return nil, common.ErrNotFound
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

// This function retrieves reference_objects tables rows with pagination. Check for issue https://github.com/0chain/gosdk/issues/117
// Might need to consider covering index for efficient search https://blog.crunchydata.com/blog/why-covering-indexes-are-incredibly-helpful
// To retrieve refs efficiently form pagination index is created in postgresql on path column so it can be used to paginate refs
// very easily and effectively; Same case for offsetDate.
func GetRefs(ctx context.Context, allocationID, path, offsetPath, _type string, level, pageLimit, offsetTime int, parentRef *PaginatedRef) (refs *[]PaginatedRef, totalPages int, newOffsetPath string, err error) {
	var (
		pRefs   = make([]PaginatedRef, 0, pageLimit/4)
		dbError error
		dbQuery *gorm.DB
	)
	path = filepath.Clean(path)
	tx := datastore.GetStore().GetTransaction(ctx)
	pathLevel := len(strings.Split(strings.TrimSuffix(path, "/"), "/"))
	if (pageLimit == 1 && offsetPath == "" && (pathLevel == level || level == 0) && _type != FILE) || (parentRef != nil && parentRef.Type == FILE) {
		pRefs = append(pRefs, *parentRef)
		refs = &pRefs
		newOffsetPath = parentRef.Path
	}

	if pathLevel+1 == level {
		dbQuery = tx.Model(&Ref{}).Where("parent_id = ?", parentRef.ID)
		if _type != "" {
			dbQuery = dbQuery.Where("type = ?", _type)
		}
		dbQuery = dbQuery.Where("path > ?", offsetPath)
		if offsetTime != 0 {
			dbQuery = dbQuery.Where("created_at < ?", offsetTime)
		}
		dbQuery = dbQuery.Order("path")
	} else {
		dbQuery = tx.Model(&Ref{}).Where("allocation_id = ? AND path LIKE ?", allocationID, path+"/%")
		if _type != "" {
			dbQuery = dbQuery.Where("type = ?", _type)
		}
		if level != 0 {
			dbQuery = dbQuery.Where("level = ?", level)
		}
		if offsetTime != 0 {
			dbQuery = dbQuery.Where("created_at < ?", offsetTime)
		}

		dbQuery = dbQuery.Where("path > ?", offsetPath)

		dbQuery = dbQuery.Order("path")
	}
	dbError = dbQuery.Limit(pageLimit).Find(&pRefs).Error
	if dbError != nil && dbError != gorm.ErrRecordNotFound {
		err = dbError
		return
	}
	refs = &pRefs
	if len(pRefs) > 0 {
		newOffsetPath = pRefs[len(pRefs)-1].Path
	}
	return
}

// Retrieves updated refs compared to some update_at value. Useful to localCache
func GetUpdatedRefs(ctx context.Context, allocationID, path, offsetPath, _type,
	updatedDate, offsetDate string, level, pageLimit int, dateLayOut string) (

	refs *[]PaginatedRef, totalPages int, newOffsetPath string,
	newOffsetDate common.Timestamp, err error) {

	var totalRows int64
	var pRefs []PaginatedRef

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
			tx := datastore.GetStore().GetTransaction(ctx)
			db1 := tx.Model(&Ref{}).Where("allocation_id = ? AND (path=? OR path LIKE ?)", allocationID, path, path+"%")
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

			return err
		})
		if err != nil {
			logging.Logger.Error("error", zap.Error(err))
		}
	}()

	go func() {
		err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
			tx := datastore.GetStore().GetTransaction(ctx)
			db2 := tx.Model(&Ref{}).Where("allocation_id = ? AND (path=? OR path LIKE ?)", allocationID, path, path+"%")
			if _type != "" {
				db2 = db2.Where("type > ?", level)
			}
			if level != 0 {
				db2 = db2.Where("level = ?", level)
			}
			if updatedDate != "" {
				db2 = db2.Where("updated_at > ?", updatedDate)
			}
			err = db2.Count(&totalRows).Error
			wg.Done()

			return err
		})
		if err != nil {
			logging.Logger.Error("error", zap.Error(err))
		}
	}()
	wg.Wait()
	if err != nil {
		return
	}

	if len(pRefs) != 0 {
		lastIdx := len(pRefs) - 1
		newOffsetDate = pRefs[lastIdx].UpdatedAt
		newOffsetPath = pRefs[lastIdx].Path
	}
	refs = &pRefs
	totalPages = int(math.Ceil(float64(totalRows) / float64(pageLimit)))
	return
}

// GetRecentlyCreatedRefs will return recently created refs with pagination. As opposed to getting
// refs ordered by path in ascending order, this will return paths in decending order for same timestamp.
// So if a file is created with path "/a/b/c/d/e/f.txt" and if "/a" didn't exist previously then
// creation date for "/a", "/a/b", "/a/b/c", "/a/b/c/d", "/a/b/c/d/e" and "/a/b/c/d/e/f.txt" will be the same.
// The refs returned will be in "/a", "/a/b", .. "/a/b/c/d/e/f.txt" order.
//
// pageLimit --> maximum number of refs to return
// fromDate --> timestamp to begin searching refs from i.e. refs created date greater than fromDate
// newOffset --> offset to use for subsequent request
func GetRecentlyCreatedRefs(
	// Note: Above mentioned function will only be feasible after splitting reference_objects table.
	// Since current limit is 50,000 files per allocation, Using common offset method should not be a big
	// deal
	ctx context.Context,
	allocID string,
	pageLimit, offset, fromDate int,
) (refs []*PaginatedRef, newOffset int, err error) {

	db := datastore.GetStore().GetTransaction(ctx)
	err = db.Model(&Ref{}).Where("allocation_id=? AND created_at > ?",
		allocID, fromDate).
		Order("created_at desc, path asc").
		Offset(offset).
		Limit(pageLimit).Find(&refs).Error

	newOffset = offset + len(refs)
	return
}

func CountRefs(ctx context.Context, allocationID string) (int64, error) {
	var totalRows int64
	tx := datastore.GetStore().GetTransaction(ctx)

	err := tx.Model(&Ref{}).
		Where("allocation_id = ?", allocationID).
		Count(&totalRows).Error

	return totalRows, err
}
