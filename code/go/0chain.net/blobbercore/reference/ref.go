package reference

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	FILE      = "f"
	DIRECTORY = "d"

	CHUNK_SIZE = 64 * 1024

	DIR_LIST_TAG  = "dirlist"
	FILE_LIST_TAG = "filelist"
)

var (
	dirListFields []string
)

func init() {
	refType := reflect.TypeOf(Ref{})

	for i := 0; i < refType.NumField(); i++ {
		field := refType.Field(i)
		dirListTag := field.Tag.Get(DIR_LIST_TAG)

		if dirListTag != "" && dirListTag != "is_empty" && dirListTag != "allocation_version" {
			dirListFields = append(dirListFields, dirListTag)
		}
	}
	dirListFields = append(dirListFields, "parent_path", "id")
}

type Ref struct {
	ID                      int64  `gorm:"column:id;primaryKey"`
	ParentID                *int64 `gorm:"column:parent_id"`
	Type                    string `gorm:"column:type;size:1" dirlist:"type" filelist:"type"`
	AllocationID            string `gorm:"column:allocation_id;size:64;not null;index:idx_path_alloc,priority:1;index:idx_parent_path_alloc,priority:1;index:idx_validation_alloc,priority:1" dirlist:"allocation_id" filelist:"allocation_id"`
	LookupHash              string `gorm:"column:lookup_hash;size:64;not null;index:idx_lookup_hash" dirlist:"lookup_hash" filelist:"lookup_hash"`
	Name                    string `gorm:"column:name;size:100;not null;index:idx_name_gin" dirlist:"name" filelist:"name"` // uses GIN tsvector index for full-text search
	Path                    string `gorm:"column:path;size:1000;not null;index:idx_path_alloc,priority:2;index:path_idx;index:idx_path_gin_trgm" dirlist:"path" filelist:"path"`
	FileMetaHash            string `gorm:"column:file_meta_hash;size:64;not null" dirlist:"file_meta_hash" filelist:"file_meta_hash"`
	NumBlocks               int64  `gorm:"column:num_of_blocks;not null;default:0" dirlist:"num_of_blocks" filelist:"num_of_blocks"`
	ParentPath              string `gorm:"column:parent_path;size:999;index:idx_parent_path_alloc,priority:2"`
	PathLevel               int    `gorm:"column:level;not null;default:0"`
	CustomMeta              string `gorm:"column:custom_meta;not null" filelist:"custom_meta" dirlist:"custom_meta"`
	Size                    int64  `gorm:"column:size;not null;default:0" dirlist:"size" filelist:"size"`
	ActualFileSize          int64  `gorm:"column:actual_file_size;not null;default:0" dirlist:"actual_file_size" filelist:"actual_file_size"`
	ActualFileHashSignature string `gorm:"column:actual_file_hash_signature;size:64" filelist:"actual_file_hash_signature"  json:"actual_file_hash_signature,omitempty"`
	ActualFileHash          string `gorm:"column:actual_file_hash;size:64;not null" filelist:"actual_file_hash"`
	MimeType                string `gorm:"column:mimetype;size:255;not null" filelist:"mimetype"`
	ThumbnailSize           int64  `gorm:"column:thumbnail_size;not null;default:0" filelist:"thumbnail_size"`
	ThumbnailHash           string `gorm:"column:thumbnail_hash;size:64;not null" filelist:"thumbnail_hash"`
	ActualThumbnailSize     int64  `gorm:"column:actual_thumbnail_size;not null;default:0" filelist:"actual_thumbnail_size"`
	ActualThumbnailHash     string `gorm:"column:actual_thumbnail_hash;size:64;not null" filelist:"actual_thumbnail_hash"`
	EncryptedKey            string `gorm:"column:encrypted_key;size:64" filelist:"encrypted_key"`
	EncryptedKeyPoint       string `gorm:"column:encrypted_key_point;size:64" filelist:"encrypted_key_point"`
	Children                []*Ref `gorm:"-"`
	childrenLoaded          bool
	CreatedAt               common.Timestamp `gorm:"column:created_at;index:idx_created_at,sort:desc" dirlist:"created_at" filelist:"created_at"`
	UpdatedAt               common.Timestamp `gorm:"column:updated_at;index:idx_updated_at,sort:desc;" dirlist:"updated_at" filelist:"updated_at"`

	DeletedAt         gorm.DeletedAt `gorm:"column:deleted_at"` // soft deletion
	ChunkSize         int64          `gorm:"column:chunk_size;not null;default:65536" dirlist:"chunk_size" filelist:"chunk_size"`
	NumUpdates        int64          `gorm:"column:num_of_updates" json:"num_of_updates"`
	NumBlockDownloads int64          `gorm:"column:num_of_block_downloads" json:"num_of_block_downloads"`
	FilestoreVersion  int            `gorm:"column:filestore_version" json:"-"`
	DataHash          string         `gorm:"column:data_hash" filelist:"data_hash"`
	DataHashSignature string         `gorm:"column:data_hash_signature" filelist:"data_hash_signature"`
	AllocationVersion int64          `gorm:"allocation_version" dirlist:"allocation_version" filelist:"allocation_version"`
	IsEmpty           bool           `gorm:"-" dirlist:"is_empty"`
	HashToBeComputed  bool           `gorm:"-"`
	prevID            int64          `gorm:"-"`
}

// BeforeCreate Hook that gets executed to update create and update date
func (ref *Ref) BeforeCreate(tx *gorm.DB) (err error) {
	if !(ref.CreatedAt > 0) {
		return fmt.Errorf("invalid timestamp value while creating for path %s", ref.Path)
	}
	if ref.UpdatedAt == 0 {
		ref.UpdatedAt = ref.CreatedAt
	}
	return nil
}

func (ref *Ref) BeforeSave(tx *gorm.DB) (err error) {
	if !(ref.UpdatedAt > 0) {
		return fmt.Errorf("invalid timestamp value while updating %s", ref.Path)
	}
	return nil
}

func (Ref) TableName() string {
	return TableNameReferenceObjects
}

type PaginatedRef struct { //Gorm smart select fields.
	ID                      int64  `gorm:"column:id" json:"id,omitempty"`
	Type                    string `gorm:"column:type" json:"type,omitempty"`
	AllocationID            string `gorm:"column:allocation_id" json:"allocation_id,omitempty"`
	LookupHash              string `gorm:"column:lookup_hash" json:"lookup_hash,omitempty"`
	Name                    string `gorm:"column:name" json:"name,omitempty"`
	Path                    string `gorm:"column:path" json:"path,omitempty"`
	NumBlocks               int64  `gorm:"column:num_of_blocks" json:"num_of_blocks,omitempty"`
	ParentPath              string `gorm:"column:parent_path" json:"parent_path,omitempty"`
	PathLevel               int    `gorm:"column:level" json:"level,omitempty"`
	CustomMeta              string `gorm:"column:custom_meta" json:"custom_meta,omitempty"`
	Size                    int64  `gorm:"column:size" json:"size,omitempty"`
	ActualFileSize          int64  `gorm:"column:actual_file_size" json:"actual_file_size,omitempty"`
	ActualFileHashSignature string `gorm:"column:actual_file_hash_signature" json:"actual_file_hash_signature,omitempty"`
	ActualFileHash          string `gorm:"column:actual_file_hash" json:"actual_file_hash,omitempty"`
	MimeType                string `gorm:"column:mimetype" json:"mimetype,omitempty"`
	ThumbnailSize           int64  `gorm:"column:thumbnail_size" json:"thumbnail_size,omitempty"`
	ThumbnailHash           string `gorm:"column:thumbnail_hash" json:"thumbnail_hash,omitempty"`
	ActualThumbnailSize     int64  `gorm:"column:actual_thumbnail_size" json:"actual_thumbnail_size,omitempty"`
	ActualThumbnailHash     string `gorm:"column:actual_thumbnail_hash" json:"actual_thumbnail_hash,omitempty"`
	EncryptedKey            string `gorm:"column:encrypted_key" json:"encrypted_key,omitempty"`
	EncryptedKeyPoint       string `gorm:"column:encrypted_key_point" json:"encrypted_key_point,omitempty"`
	FileMetaHash            string `gorm:"column:file_meta_hash;size:64;not null" json:"file_meta_hash"`

	CreatedAt common.Timestamp `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt common.Timestamp `gorm:"column:updated_at" json:"updated_at,omitempty"`

	ChunkSize int64 `gorm:"column:chunk_size" json:"chunk_size"`
}

// GetReferenceLookup hash(allocationID + ":" + path)
func GetReferenceLookup(allocationID, path string) string {
	return encryption.Hash(allocationID + ":" + path)
}

func NewDirectoryRef() *Ref {
	return &Ref{Type: DIRECTORY}
}

func NewFileRef() *Ref {
	return &Ref{Type: FILE}
}

// Mkdir create dirs if they don't exits. do nothing if dir exists. last dir will be return without child
func Mkdir(ctx context.Context, allocationID, destpath string, allocationVersion int64, ts common.Timestamp, collector QueryCollector) (*Ref, error) {
	var err error
	db := datastore.GetStore().GetTransaction(ctx)
	if destpath != "/" {
		destpath = strings.TrimSuffix(filepath.Clean("/"+destpath), "/")
	}
	destLookupHash := GetReferenceLookup(allocationID, destpath)
	var destRef *Ref
	cachedRef := collector.GetFromCache(destLookupHash)
	if cachedRef != nil {
		destRef = cachedRef
	} else {
		destRef, err = GetReferenceByLookupHashWithNewTransaction(destLookupHash)
		if err != nil && err != gorm.ErrRecordNotFound {
			return nil, err
		}
		if destRef != nil {
			destRef.LookupHash = destLookupHash
			defer collector.AddToCache(destRef)
		}
	}
	if destRef != nil {
		if destRef.Type != DIRECTORY {
			return nil, common.NewError("invalid_dir_tree", "parent path is not a directory")
		}
		return destRef, nil
	}
	fields, err := common.GetAllParentPaths(destpath)
	if err != nil {
		logging.Logger.Error("mkdir: failed to get all parent paths", zap.Error(err), zap.String("destpath", destpath))
		return nil, err
	}
	parentLookupHashes := make([]string, 0, len(fields))
	for i := 0; i < len(fields); i++ {
		parentLookupHashes = append(parentLookupHashes, GetReferenceLookup(allocationID, fields[i]))
	}
	var parentRefs []*Ref
	collector.LockTransaction()
	defer collector.UnlockTransaction()
	cachedRef = collector.GetFromCache(destLookupHash)
	if cachedRef != nil {
		if cachedRef.Type != DIRECTORY {
			return nil, common.NewError("invalid_dir_tree", "parent path is not a directory")
		}
		return cachedRef, nil
	} else {
		logging.Logger.Info("noEntryFound: ", zap.String("destLookupHash", destLookupHash), zap.String("destpath", destpath))
	}

	tx := db.Model(&Ref{}).Select("id", "path", "type")
	for i := 0; i < len(fields); i++ {
		tx = tx.Or(Ref{LookupHash: parentLookupHashes[i]})
	}
	err = tx.Order("path").Find(&parentRefs).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	var (
		parentID   int64
		parentPath = "/"
	)
	if len(parentRefs) > 0 {
		parentID = parentRefs[len(parentRefs)-1].ID
		parentPath = parentRefs[len(parentRefs)-1].Path
		for i := 0; i < len(parentRefs); i++ {
			if parentRefs[i].Type != DIRECTORY {
				return nil, common.NewError("invalid_dir_tree", "parent path is not a directory")
			}
			if parentRefs[i].ID == 0 {
				return nil, common.NewError("invalid_dir_tree", "parent path not found")
			}
		}
	}
	if destpath != "/" {
		fields = append(fields, destpath)
		parentLookupHashes = append(parentLookupHashes, destLookupHash)
	}

	for i := len(parentRefs); i < len(fields); i++ {
		logging.Logger.Info("mkdir: creating directory", zap.String("path", fields[i]), zap.Int("parentID", int(parentID)))
		var parentIDRef *int64
		if parentID > 0 {
			parentIDRef = &parentID
		} else if parentPath != "/" {
			return nil, common.NewError("invalid_dir_tree", "parent path not found")
		}
		newRef := NewDirectoryRef()
		newRef.AllocationID = allocationID
		newRef.Path = fields[i]
		if newRef.Path != "/" {
			newRef.ParentPath = parentPath
		}
		newRef.Name = filepath.Base(fields[i])
		newRef.PathLevel = i + 1
		newRef.ParentID = parentIDRef
		newRef.LookupHash = parentLookupHashes[i]
		newRef.CreatedAt = ts
		newRef.UpdatedAt = ts
		newRef.FileMetaHash = encryption.FastHash(newRef.GetFileMetaHashData())
		newRef.AllocationVersion = allocationVersion
		err = db.Create(newRef).Error
		if err != nil {
			logging.Logger.Error("mkdir: failed to create directory", zap.Error(err), zap.String("path", fields[i]))
			return nil, err
		}
		collector.AddToCache(newRef)
		parentID = newRef.ID
		parentPath = newRef.Path
	}

	dirRef := &Ref{
		AllocationID: allocationID,
		ID:           parentID,
		Path:         parentPath,
	}

	return dirRef, nil
}

// GetReference get FileRef with allcationID and path from postgres
func GetReference(ctx context.Context, allocationID, path string) (*Ref, error) {
	lookupHash := GetReferenceLookup(allocationID, path)
	return GetReferenceByLookupHash(ctx, allocationID, lookupHash)
}

// GetLimitedRefFieldsByPath get FileRef selected fields with allocationID and path from postgres
func GetLimitedRefFieldsByPath(ctx context.Context, allocationID, path string, selectedFields []string) (*Ref, error) {
	ref := &Ref{}
	t := datastore.GetStore().GetTransaction(ctx)
	db := t.Select(selectedFields)
	err := db.Where(&Ref{AllocationID: allocationID, Path: path}).Take(ref).Error
	if err != nil {
		return nil, err
	}
	return ref, nil
}

// GetLimitedRefFieldsByLookupHash get FileRef selected fields with allocationID and lookupHash from postgres
func GetLimitedRefFieldsByLookupHashWith(ctx context.Context, allocationID, lookupHash string, selectedFields []string) (*Ref, error) {
	ref := &Ref{}
	db := datastore.GetStore().GetTransaction(ctx)

	err := db.
		Select(selectedFields).
		Where(&Ref{LookupHash: lookupHash}).
		Take(ref).Error

	if err != nil {
		return nil, err
	}
	return ref, nil
}

// GetLimitedRefFieldsByLookupHash get FileRef selected fields with allocationID and lookupHash from postgres
func GetLimitedRefFieldsByLookupHash(ctx context.Context, allocationID, lookupHash string, selectedFields []string) (*Ref, error) {
	ref := &Ref{}
	t := datastore.GetStore().GetTransaction(ctx)
	db := t.Select(selectedFields)
	err := db.Where(&Ref{LookupHash: lookupHash}).Take(ref).Error
	if err != nil {
		return nil, err
	}
	return ref, nil
}

func GetReferenceByLookupHash(ctx context.Context, allocationID, pathHash string) (*Ref, error) {
	ref := &Ref{}
	db := datastore.GetStore().GetTransaction(ctx)
	err := db.Where(&Ref{LookupHash: pathHash}).Take(ref).Error
	if err != nil {
		return nil, err
	}
	return ref, nil
}

func GetPaginatedRefByLookupHash(ctx context.Context, pathHash string) (*PaginatedRef, error) {
	ref := &PaginatedRef{}
	db := datastore.GetStore().GetTransaction(ctx)
	err := db.Model(&Ref{}).Where(&Ref{LookupHash: pathHash}).Take(ref).Error
	if err != nil {
		return nil, err
	}
	return ref, nil
}

func GetReferenceByLookupHashForDownload(ctx context.Context, allocationID, pathHash string) (*Ref, error) {
	ref := &Ref{}
	db := datastore.GetStore().GetTransaction(ctx)

	err := db.Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where(&Ref{LookupHash: pathHash}).Take(ref).Error
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return ref, nil
}

func GetReferencesByName(ctx context.Context, allocationID, name string) (refs []*Ref, err error) {
	db := datastore.GetStore().GetTransaction(ctx)
	err = db.Model(&Ref{}).
		Where("allocation_id = ? AND name LIKE ?", allocationID, "%"+name+"%").
		Limit(20).
		Find(&refs).Error
	if err != nil {
		return nil, err
	}
	return refs, nil
}

// IsRefExist checks if ref with given path exists and returns error other than gorm.ErrRecordNotFound
func IsRefExist(ctx context.Context, allocationID, path string) (bool, error) {
	db := datastore.GetStore().GetTransaction(ctx)

	lookUpHash := GetReferenceLookup(allocationID, path)
	var Found bool

	err := db.Raw("SELECT EXISTS(SELECT 1 FROM reference_objects WHERE lookup_hash=? AND deleted_at is NULL) AS found", lookUpHash).Scan(&Found).Error
	if err != nil {
		return false, err
	}

	return Found, nil
}

func GetObjectSizeByLookupHash(ctx context.Context, lookupHash string) (int64, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	var size int64
	err := db.Model(&Ref{}).
		Select("size").
		Where("lookup_hash = ?", lookupHash).
		Take(&size).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
	}
	return size, nil
}

// GetRefsTypeFromPaths Give list of paths it will return refs of respective path with only Type and Path selected in sql query
func GetRefsTypeFromPaths(ctx context.Context, allocationID string, paths []string) (refs []*Ref, err error) {
	if len(paths) == 0 {
		return
	}

	t := datastore.GetStore().GetTransaction(ctx)
	db := t.Select("path", "type")
	for _, p := range paths {
		db = db.Or(Ref{AllocationID: allocationID, Path: p})
	}

	err = db.Find(&refs).Error
	return
}

func GetSubDirsFromPath(p string) []string {
	path := p
	parent, cur := filepath.Split(path)
	parent = filepath.Clean(parent)
	subDirs := make([]string, 0)
	for len(cur) > 0 {
		if cur == "." {
			break
		}
		subDirs = append([]string{cur}, subDirs...)
		parent, cur = filepath.Split(parent)
		parent = filepath.Clean(parent)
	}
	return subDirs
}

func GetRefWithChildren(ctx context.Context, parentRef *Ref, allocationID, path string, offset, pageLimit int) (*Ref, error) {
	var refs []*Ref
	t := datastore.GetStore().GetTransaction(ctx)
	db := t.Where(Ref{ParentID: &parentRef.ID})
	err := db.Order("path").
		Offset(offset).
		Limit(pageLimit).
		Find(&refs).Error
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return &Ref{Type: DIRECTORY, Path: path, AllocationID: allocationID}, nil
	}

	if parentRef.Path != path {
		return nil, common.NewError("invalid_dir_tree", "DB has invalid tree. Root not found in DB")
	}
	parentRef.Children = refs
	return parentRef, nil
}

func GetRefWithSortedChildren(ctx context.Context, allocationID, path string) (*Ref, error) {
	var refs []*Ref
	t := datastore.GetStore().GetTransaction(ctx)
	db := t.Where(
		Ref{ParentPath: path, AllocationID: allocationID}).
		Or(Ref{Type: DIRECTORY, Path: path, AllocationID: allocationID})

	err := db.Order("path").Find(&refs).Error
	if err != nil {
		return nil, err
	}

	if len(refs) == 0 {
		return &Ref{Type: DIRECTORY, Path: path, AllocationID: allocationID}, nil
	}

	curRef := refs[0]
	if curRef.Path != path {
		return nil, common.NewError("invalid_dir_tree", "DB has invalid tree. Root not found in DB")
	}

	for i := 1; i < len(refs); i++ {
		if refs[i].ParentPath == curRef.Path {
			curRef.Children = append(curRef.Children, refs[i])
		} else {
			return nil, common.NewError("invalid_dir_tree", "DB has invalid tree.")
		}
	}

	return refs[0], nil
}

func GetRefWithDirListFields(ctx context.Context, pathHash string) (*Ref, error) {
	ref := &Ref{}
	// get all ref fields with dirlist tag
	db := datastore.GetStore().GetTransaction(ctx)
	err := db.Select(dirListFields).
		Where(&Ref{LookupHash: pathHash}).
		Take(ref).Error
	if err != nil {
		return nil, err
	}
	return ref, nil
}

func (r *Ref) GetFileMetaHashData() string {
	return fmt.Sprintf(
		"%s:%d:%d:%s",
		r.Path, r.Size,
		r.ActualFileSize, r.ActualFileHash)
}

func (fr *Ref) GetFileHashData() string {
	return fmt.Sprintf(
		"%s:%s:%s:%s:%d:%d:%s:%d",
		fr.AllocationID,
		fr.Type, // don't need to add it as well
		fr.Name, // don't see any utility as fr.Path below has name in it
		fr.Path,
		fr.Size,
		fr.ActualFileSize,
		fr.ActualFileHash,
		fr.ChunkSize,
	)
}

func (r *Ref) GetHashData() string {
	return fmt.Sprintf("%s:%s", r.AllocationID, r.Path)
}

func (fr *Ref) CalculateFileHash(ctx context.Context, saveToDB bool, collector QueryCollector) (string, error) {
	fr.FileMetaHash = encryption.Hash(fr.GetFileMetaHashData())
	fr.NumBlocks = int64(math.Ceil(float64(fr.Size*1.0) / float64(fr.ChunkSize)))
	fr.PathLevel = len(strings.Split(strings.TrimRight(fr.Path, "/"), "/"))
	fr.LookupHash = GetReferenceLookup(fr.AllocationID, fr.Path)

	var err error
	if saveToDB && fr.HashToBeComputed {
		err = fr.SaveFileRef(ctx, collector)
	}
	return fr.FileMetaHash, err
}

func (r *Ref) CalculateDirHash(ctx context.Context, saveToDB bool, collector QueryCollector) (h string, err error) {
	if !r.HashToBeComputed {
		h = r.FileMetaHash
		return
	}

	l := len(r.Children)

	defer func() {
		if err == nil && saveToDB {
			err = r.SaveDirRef(ctx, collector)

		}
	}()

	childFileMetaHashes := make([]string, l)
	var refNumBlocks, size, actualSize int64

	for i, childRef := range r.Children {
		if childRef.HashToBeComputed {
			_, err := childRef.CalculateHash(ctx, saveToDB, collector)
			if err != nil {
				return "", err
			}
		}

		childFileMetaHashes[i] = childRef.FileMetaHash
		refNumBlocks += childRef.NumBlocks
		size += childRef.Size
		actualSize += childRef.ActualFileSize
	}

	r.FileMetaHash = encryption.Hash(r.Path + strings.Join(childFileMetaHashes, ":"))
	r.NumBlocks = refNumBlocks
	r.Size = size
	r.ActualFileSize = actualSize
	r.PathLevel = len(GetSubDirsFromPath(r.Path)) + 1
	r.LookupHash = GetReferenceLookup(r.AllocationID, r.Path)
	return r.FileMetaHash, err
}

func (r *Ref) CalculateHash(ctx context.Context, saveToDB bool, collector QueryCollector) (string, error) {
	if r.Type == DIRECTORY {
		return r.CalculateDirHash(ctx, saveToDB, collector)
	}
	return r.CalculateFileHash(ctx, saveToDB, collector)
}

func (r *Ref) AddChild(child *Ref) {
	if r.Children == nil {
		r.Children = make([]*Ref, 0)
	}
	r.childrenLoaded = true
	var index int
	var ltFound bool
	// Add child in sorted fashion
	for i, ref := range r.Children {
		if strings.Compare(child.Name, ref.Name) == 0 {
			r.Children[i] = child

			return
		}
		if child.ParentPath != ref.ParentPath {
			logging.Logger.Error("invalid parent path", zap.String("child", child.Path), zap.String("parent", ref.Path))
		}
		if strings.Compare(child.Path, ref.Path) == -1 {
			index = i
			ltFound = true
			break
		}
	}
	if ltFound {
		r.Children = append(r.Children[:index+1], r.Children[index:]...)
		r.Children[index] = child
	} else {
		r.Children = append(r.Children, child)
	}
}

func (r *Ref) RemoveChild(idx int) {
	if idx < 0 {
		return
	}
	r.Children = append(r.Children[:idx], r.Children[idx+1:]...)
	r.childrenLoaded = true
}

func (r *Ref) UpdatePath(newPath, parentPath string) {
	r.Path = newPath
	r.ParentPath = parentPath
	r.PathLevel = len(GetSubDirsFromPath(r.Path)) + 1
	r.LookupHash = GetReferenceLookup(r.AllocationID, r.Path)
}

func (r *Ref) SaveFileRef(ctx context.Context, collector QueryCollector) error {
	r.prevID = r.ID
	r.NumUpdates += 1
	if r.ID > 0 {
		deleteRef := &Ref{ID: r.ID}
		collector.DeleteRefRecord(deleteRef)
		r.ID = 0
	}
	collector.CreateRefRecord(r)

	return nil
}

func (r *Ref) SaveDirRef(ctx context.Context, collector QueryCollector) error {
	r.prevID = r.ID
	r.NumUpdates += 1
	if r.ID > 0 {
		deleteRef := &Ref{ID: r.ID}
		collector.DeleteRefRecord(deleteRef)
		r.ID = 0
	}
	collector.CreateRefRecord(r)
	return nil
}

func (r *Ref) Save(ctx context.Context) error {
	db := datastore.GetStore().GetTransaction(ctx)
	return db.Save(r).Error
}

// GetListingData reflect and convert all fields into map[string]interface{}
func (r *Ref) GetListingData(ctx context.Context) map[string]interface{} {
	if r == nil {
		return make(map[string]interface{})
	}

	if r.Type == FILE {
		return GetListingFieldsMap(*r, FILE_LIST_TAG)
	}
	return GetListingFieldsMap(*r, DIR_LIST_TAG)
}

func ListingDataToRef(refMap map[string]interface{}) *Ref {
	if len(refMap) < 1 {
		return nil
	}
	ref := &Ref{}

	refType, _ := refMap["type"].(string)
	var tagName string
	if refType == FILE {
		tagName = FILE_LIST_TAG
	} else {
		tagName = DIR_LIST_TAG
	}

	t := reflect.TypeOf(ref).Elem()
	v := reflect.ValueOf(ref).Elem()

	// Iterate over all available fields and read the tag value
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Get the field tag value
		tag := field.Tag.Get(tagName)
		// Skip if tag is not defined or ignored
		if tag == "" || tag == "-" {
			continue
		}

		val := refMap[tag]
		if val != nil {
			v.FieldByName(field.Name).Set(reflect.ValueOf(val))
		}
	}

	return ref
}

func GetListingFieldsMap(refEntity interface{}, tagName string) map[string]interface{} {
	result := make(map[string]interface{})
	t := reflect.TypeOf(refEntity)
	v := reflect.ValueOf(refEntity)
	// Iterate over all available fields and read the tag value
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Get the field tag value
		tag := field.Tag.Get(tagName)
		// Skip if tag is not defined or ignored
		if !field.Anonymous && (tag == "" || tag == "-") {
			continue
		}

		if field.Anonymous {
			listMap := GetListingFieldsMap(v.FieldByName(field.Name).Interface(), tagName)
			if len(listMap) > 0 {
				for k, v := range listMap {
					result[k] = v
				}
			}
		} else {
			fieldValue := v.FieldByName(field.Name).Interface()
			if fieldValue == nil {
				continue
			}
			result[tag] = fieldValue
		}
	}
	return result
}

func UpdateCustomMeta(ctx context.Context, ref *Ref, customMeta string) error {
	db := datastore.GetStore().GetTransaction(ctx)
	return db.Exec("UPDATE reference_objects SET custom_meta = ? WHERE id = ?", customMeta, ref.ID).Error
}

func IsDirectoryEmpty(ctx context.Context, id int64) (bool, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	var ref Ref
	err := db.Model(&Ref{}).Select("id").Where("parent_id = ?", &id).Take(&ref).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return true, nil
		}
		return false, err
	}
	if ref.ID > 0 {
		return false, nil
	}

	return true, nil
}

func GetReferenceByLookupHashWithNewTransaction(lookupHash string) (*Ref, error) {
	var ref *Ref
	err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		txn := datastore.GetStore().GetTransaction(ctx)
		return txn.Model(&Ref{}).Select("id", "type").Where("lookup_hash = ?", lookupHash).Take(&ref).Error
	}, &sql.TxOptions{
		ReadOnly: true,
	})
	if err != nil {
		return nil, err
	}
	return ref, nil
}

func GetFullReferenceByLookupHashWithNewTransaction(lookupHash string) (*Ref, error) {
	var ref *Ref
	err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		txn := datastore.GetStore().GetTransaction(ctx)
		return txn.Model(&Ref{}).Where("lookup_hash = ?", lookupHash).Take(&ref).Error
	}, &sql.TxOptions{
		ReadOnly: true,
	})
	if err != nil {
		return nil, err
	}
	return ref, nil
}
