package reference

import (
	"strconv"
	"strings"

	"gorm.io/datatypes"
)

// HashNode ref node in hash tree
type HashNode struct {
	// hash data
	AllocationID   string         `gorm:"column:allocation_id" json:"allocation_id,omitempty"`
	Type           string         `gorm:"column:type" json:"type,omitempty"`
	Name           string         `gorm:"column:name" json:"name,omitempty"`
	Path           string         `gorm:"column:path" json:"path,omitempty"`
	ContentHash    string         `gorm:"column:content_hash" json:"content_hash,omitempty"`
	MerkleRoot     string         `gorm:"column:merkle_root" json:"merkle_root,omitempty"`
	ActualFileHash string         `gorm:"column:actual_file_hash" json:"actual_file_hash,omitempty"`
	Attributes     datatypes.JSON `gorm:"column:attributes" json:"attributes,omitempty"`
	ChunkSize      int64          `gorm:"column:chunk_size" json:"chunk_size,omitempty"`
	Size           int64          `gorm:"column:size" json:"size,omitempty"`
	ActualFileSize int64          `gorm:"column:actual_file_size" json:"actual_file_size,omitempty"`

	// other data
	ParentPath string      `gorm:"parent_path" json:"parent_path,omitempty"`
	Children   []*HashNode `gorm:"-", json:"children"`
}

// TableName get table name of Ref
func (HashNode) TableName() string {
	return TableNameReferenceObjects
}

func (n *HashNode) AddChild(c *HashNode) {
	if n.Children == nil {
		n.Children = make([]*HashNode, 0, 10)
	}

	n.Children = append(n.Children, c)
}

// GetLookupHash get lookuphash
func (n *HashNode) GetLookupHash() string {
	return GetReferenceLookup(n.AllocationID, n.Path)
}

// GetHashCode get hash code
func (n *HashNode) GetHashCode() string {

	if len(n.Attributes) == 0 {
		n.Attributes = datatypes.JSON("{}")
	}
	hashArray := []string{
		n.AllocationID,
		n.Type,
		n.Name,
		n.Path,
		strconv.FormatInt(n.Size, 10),
		n.ContentHash,
		n.MerkleRoot,
		strconv.FormatInt(n.ActualFileSize, 10),
		n.ActualFileHash,
		string(n.Attributes),
		strconv.FormatInt(n.ChunkSize, 10),
	}

	return strings.Join(hashArray, ":")
}
