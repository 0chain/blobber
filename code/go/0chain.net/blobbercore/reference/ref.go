package reference

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	FILE      = "f"
	DIRECTORY = "d"

	CHUNK_SIZE = 64 * 1024

	DIR_LIST_TAG  = "dirlist"
	FILE_LIST_TAG = "filelist"
)

// The Attributes represents file attributes.
type Attributes struct {
	// The WhoPaysForReads represents reading payer. It can be allocation owner
	// or a 3rd party user. It affects read operations only. It requires
	// blobbers to be trusted.
	WhoPaysForReads common.WhoPays `json:"who_pays_for_reads,omitempty"`

	// add more file / directory attributes by needs with
	// 'omitempty' json tag to avoid hash difference for
	// equal values
}

// IsZero returns true, if the Attributes is zero.
func (a *Attributes) IsZero() bool {
	return (*a) == (Attributes{})
}

// Validate the Attributes.
func (a *Attributes) Validate() (err error) {
	if err = a.WhoPaysForReads.Validate(); err != nil {
		return common.NewErrorf("validating_object_attributes",
			"invalid who_pays_for_reads field: %v", err)
	}
	return
}

type Ref struct {
	ID                  int64          `gorm:"column:id;primary_key"`
	Type                string         `gorm:"column:type" dirlist:"type" filelist:"type"`
	AllocationID        string         `gorm:"column:allocation_id"`
	LookupHash          string         `gorm:"column:lookup_hash" dirlist:"lookup_hash" filelist:"lookup_hash"`
	Name                string         `gorm:"column:name" dirlist:"name" filelist:"name"`
	Path                string         `gorm:"column:path" dirlist:"path" filelist:"path"`
	Hash                string         `gorm:"column:hash" dirlist:"hash" filelist:"hash"`
	NumBlocks           int64          `gorm:"column:num_of_blocks" dirlist:"num_of_blocks" filelist:"num_of_blocks"`
	PathHash            string         `gorm:"column:path_hash" dirlist:"path_hash" filelist:"path_hash"`
	ParentPath          string         `gorm:"column:parent_path"`
	PathLevel           int            `gorm:"column:level"`
	CustomMeta          string         `gorm:"column:custom_meta" filelist:"custom_meta"`
	ContentHash         string         `gorm:"column:content_hash" filelist:"content_hash"`
	Size                int64          `gorm:"column:size" dirlist:"size" filelist:"size"`
	MerkleRoot          string         `gorm:"column:merkle_root" filelist:"merkle_root"`
	ActualFileSize      int64          `gorm:"column:actual_file_size" filelist:"actual_file_size"`
	ActualFileHash      string         `gorm:"column:actual_file_hash" filelist:"actual_file_hash"`
	MimeType            string         `gorm:"column:mimetype" filelist:"mimetype"`
	WriteMarker         string         `gorm:"column:write_marker"`
	ThumbnailSize       int64          `gorm:"column:thumbnail_size" filelist:"thumbnail_size"`
	ThumbnailHash       string         `gorm:"column:thumbnail_hash" filelist:"thumbnail_hash"`
	ActualThumbnailSize int64          `gorm:"column:actual_thumbnail_size" filelist:"actual_thumbnail_size"`
	ActualThumbnailHash string         `gorm:"column:actual_thumbnail_hash" filelist:"actual_thumbnail_hash"`
	EncryptedKey        string         `gorm:"column:encrypted_key" filelist:"encrypted_key"`
	Attributes          datatypes.JSON `gorm:"column:attributes" filelist:"attributes"`
	Children            []*Ref         `gorm:"-"`
	childrenLoaded      bool

	OnCloud        bool            `gorm:"column:on_cloud" filelist:"on_cloud"`
	CommitMetaTxns []CommitMetaTxn `gorm:"foreignkey:ref_id" filelist:"commit_meta_txns"`
	CreatedAt      time.Time       `gorm:"column:created_at" dirlist:"created_at" filelist:"created_at"`
	UpdatedAt      time.Time       `gorm:"column:updated_at" dirlist:"updated_at" filelist:"updated_at"`

	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at"` // soft deletion

	ChunkSize int64 `gorm:"column:chunk_size" dirlist:"chunk_size" filelist:"chunk_size"`
}

type PaginatedRef struct { //Gorm smart select fields.
	ID                  int64          `gorm:"column:id" json:"id,omitempty"`
	Type                string         `gorm:"column:type" json:"type,omitempty"`
	AllocationID        string         `gorm:"column:allocation_id" json:"allocation_id,omitempty"`
	LookupHash          string         `gorm:"column:lookup_hash" json:"lookup_hash,omitempty"`
	Name                string         `gorm:"column:name" json:"name,omitempty"`
	Path                string         `gorm:"column:path" json:"path,omitempty"`
	Hash                string         `gorm:"column:hash" json:"hash,omitempty"`
	NumBlocks           int64          `gorm:"column:num_of_blocks" json:"num_blocks,omitempty"`
	PathHash            string         `gorm:"column:path_hash" json:"path_hash,omitempty"`
	ParentPath          string         `gorm:"column:parent_path" json:"parent_path,omitempty"`
	PathLevel           int            `gorm:"column:level" json:"level,omitempty"`
	CustomMeta          string         `gorm:"column:custom_meta" json:"custom_meta,omitempty"`
	ContentHash         string         `gorm:"column:content_hash" json:"content_hash,omitempty"`
	Size                int64          `gorm:"column:size" json:"size,omitempty"`
	MerkleRoot          string         `gorm:"column:merkle_root" json:"merkle_root,omitempty"`
	ActualFileSize      int64          `gorm:"column:actual_file_size" json:"actual_file_size,omitempty"`
	ActualFileHash      string         `gorm:"column:actual_file_hash" json:"actual_file_hash,omitempty"`
	MimeType            string         `gorm:"column:mimetype" json:"mimetype,omitempty"`
	WriteMarker         string         `gorm:"column:write_marker" json:"write_marker,omitempty"`
	ThumbnailSize       int64          `gorm:"column:thumbnail_size" json:"thumbnail_size,omitempty"`
	ThumbnailHash       string         `gorm:"column:thumbnail_hash" json:"thumbnail_hash,omitempty"`
	ActualThumbnailSize int64          `gorm:"column:actual_thumbnail_size" json:"actual_thumbnail_size,omitempty"`
	ActualThumbnailHash string         `gorm:"column:actual_thumbnail_hash" json:"actual_thumbnail_hash,omitempty"`
	EncryptedKey        string         `gorm:"column:encrypted_key" json:"encrypted_key,omitempty"`
	Attributes          datatypes.JSON `gorm:"column:attributes" json:"attributes,omitempty"`

	OnCloud   bool           `gorm:"column:on_cloud" json:"on_cloud,omitempty"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updated_at,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at" json:"-"` // soft deletion

	ChunkSize int64 `gorm:"column:chunk_size" dirlist:"chunk_size" filelist:"chunk_size"`
}

func (Ref) TableName() string {
	return "reference_objects"
}

// GetReferenceLookup hash(allocationID + ":" + path)
func GetReferenceLookup(allocationID string, path string) string {
	return encryption.Hash(allocationID + ":" + path)
}

func NewDirectoryRef() *Ref {
	return &Ref{Type: DIRECTORY, Attributes: datatypes.JSON("{}")}
}

func NewFileRef() *Ref {
	return &Ref{Type: FILE, Attributes: datatypes.JSON("{}")}
}

func (r *Ref) GetAttributes() (attr *Attributes, err error) {
	if len(r.Attributes) == 0 {
		attr = new(Attributes) // zero attributes
		return
	}
	attr = new(Attributes)
	if err = json.Unmarshal([]byte(r.Attributes), attr); err != nil {
		return nil, common.NewError("decoding file attributes", err.Error())
	}
	return // the decoded attributes
}

func (r *Ref) SetAttributes(attr *Attributes) (err error) {
	if attr == nil || (*attr) == (Attributes{}) {
		r.Attributes = datatypes.JSON("{}") // use zero value
		return
	}
	var b []byte
	if b, err = json.Marshal(attr); err != nil {
		return common.NewError("encoding file attributes", err.Error())
	}
	r.Attributes = datatypes.JSON(b) // or a real value, can be {} too
	return
}

// Mkdir create dirs if they don't exits. do nothing if dir exists. last dir will be returned
func Mkdir(ctx context.Context, allocationID string, destpath string) (*Ref, error) {
	var dirRef *Ref
	db := datastore.GetStore().GetTransaction(ctx)
	// cleaning path to avoid edge case issues: append '/' prefix if not added and removing suffix '/' if added
	destpath = strings.TrimSuffix(filepath.Clean("/"+destpath), "/")
	dirs := strings.Split(destpath, "/")

	for i := range dirs {
		currentPath := filepath.Join("/", filepath.Join(dirs[:i+1]...))
		ref, err := GetReference(ctx, allocationID, currentPath)
		if err == nil {
			ref.AddChild(dirRef)
			dirRef = ref
			continue
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			// unexpected sql error
			return nil, err
		}

		// dir doesn't exists , create it
		newRef := NewDirectoryRef()
		newRef.AllocationID = allocationID
		newRef.Path = currentPath
		newRef.ParentPath = filepath.Join("/", filepath.Join(dirs[:i]...))
		newRef.Name = dirs[i]
		newRef.Type = DIRECTORY
		newRef.PathLevel = i + 1
		newRef.LookupHash = GetReferenceLookup(allocationID, newRef.Path)
		err = db.Create(newRef).Error
		if err != nil {
			return nil, err
		}

		newRef.AddChild(dirRef)
		dirRef = newRef
	}

	return dirRef, nil

}

// GetReference get FileRef with allcationID and path from postgres
func GetReference(ctx context.Context, allocationID string, path string) (*Ref, error) {
	ref := &Ref{}
	db := datastore.GetStore().GetTransaction(ctx)
	err := db.Where(&Ref{AllocationID: allocationID, Path: path}).First(ref).Error
	if err == nil {
		return ref, nil
	}
	return nil, err
}

func GetReferenceFromLookupHash(ctx context.Context, allocationID string, path_hash string) (*Ref, error) {
	ref := &Ref{}
	db := datastore.GetStore().GetTransaction(ctx)
	err := db.Where(&Ref{AllocationID: allocationID, LookupHash: path_hash}).First(ref).Error
	if err == nil {
		return ref, nil
	}
	return nil, err
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

func GetRefWithChildren(ctx context.Context, allocationID string, path string) (*Ref, error) {
	var refs []Ref
	db := datastore.GetStore().GetTransaction(ctx)
	db = db.Where(Ref{ParentPath: path, AllocationID: allocationID}).Or(Ref{Type: DIRECTORY, Path: path, AllocationID: allocationID})
	err := db.Order("level, created_at").Find(&refs).Error
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return &Ref{Type: DIRECTORY, Path: path, AllocationID: allocationID}, nil
	}
	curRef := &refs[0]
	if curRef.Path != path {
		return nil, common.NewError("invalid_dir_tree", "DB has invalid tree. Root not found in DB")
	}
	for i := 1; i < len(refs); i++ {
		if refs[i].ParentPath == curRef.Path {
			curRef.Children = append(curRef.Children, &refs[i])
		} else {
			return nil, common.NewError("invalid_dir_tree", "DB has invalid tree.")
		}
	}
	return &refs[0], nil
}

func GetRefWithSortedChildren(ctx context.Context, allocationID string, path string) (*Ref, error) {
	var refs []*Ref
	db := datastore.GetStore().GetTransaction(ctx)
	db = db.Where(Ref{ParentPath: path, AllocationID: allocationID}).Or(Ref{Type: DIRECTORY, Path: path, AllocationID: allocationID})
	err := db.Order("level, lookup_hash").Find(&refs).Error
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

func (fr *Ref) GetFileHashData() string {
	if len(fr.Attributes) == 0 {
		fr.Attributes = datatypes.JSON("{}")
	}
	hashArray := make([]string, 0, 11)
	hashArray = append(hashArray, fr.AllocationID)
	hashArray = append(hashArray, fr.Type)
	hashArray = append(hashArray, fr.Name)
	hashArray = append(hashArray, fr.Path)
	hashArray = append(hashArray, strconv.FormatInt(fr.Size, 10))
	hashArray = append(hashArray, fr.ContentHash)
	hashArray = append(hashArray, fr.MerkleRoot)
	hashArray = append(hashArray, strconv.FormatInt(fr.ActualFileSize, 10))
	hashArray = append(hashArray, fr.ActualFileHash)
	hashArray = append(hashArray, string(fr.Attributes))
	hashArray = append(hashArray, strconv.FormatInt(fr.ChunkSize, 10))

	return strings.Join(hashArray, ":")
}

func (fr *Ref) CalculateFileHash(ctx context.Context, saveToDB bool) (string, error) {
	fr.Hash = encryption.Hash(fr.GetFileHashData())
	fr.NumBlocks = int64(math.Ceil(float64(fr.Size*1.0) / float64(fr.ChunkSize)))
	fr.PathHash = GetReferenceLookup(fr.AllocationID, fr.Path)
	fr.PathLevel = len(GetSubDirsFromPath(fr.Path)) + 1
	fr.LookupHash = GetReferenceLookup(fr.AllocationID, fr.Path)
	var err error
	if saveToDB {
		err = fr.Save(ctx)
	}
	return fr.Hash, err
}

func (r *Ref) CalculateDirHash(ctx context.Context, saveToDB bool) (string, error) {
	// empty directory, return hash directly
	if len(r.Children) == 0 && !r.childrenLoaded {
		return r.Hash, nil
	}
	sort.SliceStable(r.Children, func(i, j int) bool {
		return strings.Compare(r.Children[i].LookupHash, r.Children[j].LookupHash) == -1
	})
	for _, childRef := range r.Children {
		_, err := childRef.CalculateHash(ctx, saveToDB)
		if err != nil {
			return "", err
		}
	}
	childHashes := make([]string, len(r.Children))
	childPathHashes := make([]string, len(r.Children))
	var refNumBlocks int64
	var size int64
	for index, childRef := range r.Children {
		childHashes[index] = childRef.Hash
		childPathHashes[index] = childRef.PathHash
		refNumBlocks += childRef.NumBlocks
		size += childRef.Size
	}

	r.Hash = encryption.Hash(strings.Join(childHashes, ":"))

	r.NumBlocks = refNumBlocks
	r.Size = size
	r.PathHash = encryption.Hash(strings.Join(childPathHashes, ":"))
	r.PathLevel = len(GetSubDirsFromPath(r.Path)) + 1
	r.LookupHash = GetReferenceLookup(r.AllocationID, r.Path)

	var err error
	if saveToDB {
		err = r.Save(ctx)
	}

	return r.Hash, err
}

func (r *Ref) CalculateHash(ctx context.Context, saveToDB bool) (string, error) {
	if r.Type == DIRECTORY {
		return r.CalculateDirHash(ctx, saveToDB)
	}
	return r.CalculateFileHash(ctx, saveToDB)
}

func (r *Ref) AddChild(child *Ref) {
	if r.Children == nil {
		r.Children = make([]*Ref, 0)
	}
	r.Children = append(r.Children, child)
	sort.SliceStable(r.Children, func(i, j int) bool {
		return strings.Compare(r.Children[i].LookupHash, r.Children[j].LookupHash) == -1
	})
	r.childrenLoaded = true
}

func (r *Ref) RemoveChild(idx int) {
	if idx < 0 {
		return
	}
	r.Children = append(r.Children[:idx], r.Children[idx+1:]...)
	sort.SliceStable(r.Children, func(i, j int) bool {
		return strings.Compare(r.Children[i].LookupHash, r.Children[j].LookupHash) == -1
	})
	r.childrenLoaded = true
}

func (r *Ref) UpdatePath(newPath string, parentPath string) {
	r.Path = newPath
	r.ParentPath = parentPath
	r.PathLevel = len(GetSubDirsFromPath(r.Path)) + 1 //strings.Count(r.Path, "/")
	r.LookupHash = GetReferenceLookup(r.AllocationID, r.Path)
}

func DeleteReference(ctx context.Context, refID int64, pathHash string) error {
	if refID <= 0 {
		return common.NewError("invalid_ref_id", "Invalid reference ID to delete")
	}
	db := datastore.GetStore().GetTransaction(ctx)
	return db.Where("path_hash = ?", pathHash).Delete(&Ref{ID: refID}).Error
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
