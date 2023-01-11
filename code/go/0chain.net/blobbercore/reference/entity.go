package reference

// Hashnode ref node in hash tree
type Hashnode struct {
	// hash data
	AllocationID    string `gorm:"column:allocation_id" json:"allocation_id,omitempty"`
	Type            string `gorm:"column:type" json:"type,omitempty"`
	Name            string `gorm:"column:name" json:"name,omitempty"`
	Path            string `gorm:"column:path" json:"path,omitempty"`
	ValidationRoot  string `gorm:"column:validation_root" json:"validation_root,omitempty"`
	FixedMerkleRoot string `gorm:"column:fixed_merkle_root" json:"fixed_merkle_root,omitempty"`
	ActualFileHash  string `gorm:"column:actual_file_hash" json:"actual_file_hash,omitempty"`
	ChunkSize       int64  `gorm:"column:chunk_size" json:"chunk_size,omitempty"`
	Size            int64  `gorm:"column:size" json:"size,omitempty"`
	ActualFileSize  int64  `gorm:"column:actual_file_size" json:"actual_file_size,omitempty"`

	// other data
	ParentPath string      `gorm:"parent_path" json:"-"`
	Children   []*Hashnode `gorm:"-" json:"children,omitempty"`
}

// TableName get table name of Ref
func (Hashnode) TableName() string {
	return TableNameReferenceObjects
}

func (n *Hashnode) AddChild(c *Hashnode) {
	if n.Children == nil {
		n.Children = make([]*Hashnode, 0, 10)
	}

	n.Children = append(n.Children, c)
}

// GetLookupHash get lookuphash
func (n *Hashnode) GetLookupHash() string {
	return GetReferenceLookup(n.AllocationID, n.Path)
}
