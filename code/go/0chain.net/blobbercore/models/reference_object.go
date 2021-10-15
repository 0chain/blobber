package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const TableNameReferenceObject = "reference_objects"

type ReferenceObject struct {
	ID                  int64          `gorm:"column:id;primary_key"`
	Type                string         `gorm:"column:type"`
	AllocationID        string         `gorm:"column:allocation_id"`
	LookupHash          string         `gorm:"column:lookup_hash"`
	Name                string         `gorm:"column:name"`
	Path                string         `gorm:"column:path"`
	Hash                string         `gorm:"column:hash"`
	NumBlocks           int64          `gorm:"column:num_of_blocks"`
	PathHash            string         `gorm:"column:path_hash"`
	ParentPath          string         `gorm:"column:parent_path"`
	PathLevel           int            `gorm:"column:level"`
	CustomMeta          string         `gorm:"column:custom_meta"`
	ContentHash         string         `gorm:"column:content_hash"`
	Size                int64          `gorm:"column:size"`
	MerkleRoot          string         `gorm:"column:merkle_root"`
	ActualFileSize      int64          `gorm:"column:actual_file_size"`
	ActualFileHash      string         `gorm:"column:actual_file_hash"`
	MimeType            string         `gorm:"column:mimetype"`
	WriteMarker         string         `gorm:"column:write_marker"`
	ThumbnailSize       int64          `gorm:"column:thumbnail_size"`
	ThumbnailHash       string         `gorm:"column:thumbnail_hash"`
	ActualThumbnailSize int64          `gorm:"column:actual_thumbnail_size"`
	ActualThumbnailHash string         `gorm:"column:actual_thumbnail_hash"`
	EncryptedKey        string         `gorm:"column:encrypted_key"`
	Attributes          datatypes.JSON `gorm:"column:attributes"`

	OnCloud bool `gorm:"column:on_cloud"`

	CreatedAt time.Time      `gorm:"column:created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at"` // soft deletion

	ChunkSize int64 `gorm:"column:chunk_size"`
}

func (ReferenceObject) TableName() string {
	return TableNameReferenceObject
}
