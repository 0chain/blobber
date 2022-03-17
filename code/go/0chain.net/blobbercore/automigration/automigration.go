//This file is used to create table schemas using gorm's automigration feature which takes information from
//struct's fields and functions

package automigration

import (
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/challenge"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

//Create indexes
// ref: https://gorm.io/docs/indexes.html
// We can create unique index, multiple indexes, composite indexes with priority set

// reference_objects table
type ReferenceObjectsWithIndexes struct {
	reference.Ref
	Path         string    `gorm:"column:path;index:idx_path_alloc,priority:2;index:path_idx"`
	LookupHash   string    `gorm:"column:lookup_hash;index:idx_lookup_hash_alloc,priority:2"`
	AllocationID string    `gorm:"column:allocation_id;index:idx_path_alloc,priority:1;index:idx_lookup_hash_alloc,priority:1"`
	UpdatedAt    time.Time `gorm:"column:updated_at;index:idx_updated_at"`
}

// allocations table
type AllocationsWithIndexes struct {
	allocation.Allocation
	Tx string `gorm:"column:tx;index:idx_unique_allocations_tx,unique"`
}

// pendings table
type PendingsWithIndexes struct {
	allocation.Pending
	ClientID     string `gorm:"column:client_id;index:idx_pendings_cab,priority:1"`
	AllocationID string `gorm:"column:allocation_id;index:idx_pendings_cab,priority:2"`
	BlobberID    string `gorm:"column:blobber_id;index:idx_pendings_cab,priority:3"`
}

// read_pools table
type ReadPoolsWithIndexes struct {
	allocation.ReadPool
	ClientID     string `gorm:"column:client_id;index:idx_read_pools_cab,priority:1"`
	AllocationID string `gorm:"column:allocation_id;index:idx_read_pools_cab,priority:2"`
	BlobberID    string `gorm:"column:blobber_id;index:idx_read_pools_cab,priority:3"`
}

// write_pools table
type WritePoolsWithIndexes struct {
	allocation.WritePool
	ClientID     string `gorm:"column:client_id;index:idx_write_pools_cab,priority:1"`
	AllocationID string `gorm:"column:allocation_id;index:idx_write_pools_cab,priority:2"`
	BlobberID    string `gorm:"column:blobber_id;index:idx_write_pools_cab,priority:3"`
}

// marketplace_share_info table
type ShareInfoWithIndexes struct {
	reference.ShareInfo
	OwnerID      string `gorm:"owner_id;index:idx_marketplace_share_info_for_owner,priority:1"`
	ClientID     string `gorm:"client_id;index:idx_marketplace_share_info_for_client,priority:1"`
	FilePathHash string `gorm:"file_path_hash;index:idx_marketplace_share_info_for_owner,priority:2;index:idx_marketplace_share_info_for_client,priority:2"`
}

var tables = []interface{}{
	getReferenceObjectModel(),
	new(reference.CommitMetaTxn),
	new(reference.ShareInfo),
	new(challenge.ChallengeEntity),
	new(allocation.Allocation),
	new(allocation.AllocationChange),
	new(allocation.AllocationChangeCollector),
	new(allocation.Pending),
	new(allocation.Terms),
	new(allocation.ReadPool),
	new(allocation.WritePool),
	new(readmarker.ReadMarkerEntity),
	new(writemarker.WriteMarkerEntity),
	new(writemarker.WriteLock),
	new(stats.FileStats),
}

func AutomigrateSchema(db *gorm.DB) error {
	if err := db.AutoMigrate(tables...); err != nil {
		return err
	}

	return nil
}

func getReferenceObjectModel() interface{} {
	type Ref struct {
		reference.Ref
		ID                  int64          `gorm:"column:id;primary_key"`
		Type                string         `gorm:"column;size:1"`
		AllocationID        string         `gorm:"column:allocation_id;size:64;not null;index:idx_path_alloc,priority:1;index:idx_lookup_hash_alloc,priority:1"`
		LookupHash          string         `gorm:"column:lookup_hash;size:64;not null;index:idx_lookup_hash_alloc,priority:2"`
		Name                string         `gorm:"column:name;size:255;not null"`
		Path                string         `gorm:"column:path;size:255;not null;index:idx_path_alloc,priority:2;index:path_idx"`
		Hash                string         `gorm:"column:hash;size:64;not null"`
		NumBlocks           int64          `gorm:"column:num_of_blocks;not null;default:0"`
		PathHash            string         `gorm:"column:path_hash;size:64;not null"`
		ParentPath          string         `gorm:"column:parent_path;size:255"`
		PathLevel           int8           `gorm:"column:level;not null;default:0"`
		CustomMeta          string         `gorm:"column:custom_meta;not null"`
		ContentHash         string         `gorm:"column:content_hash;size:64;not null"`
		Size                int64          `gorm:"column:size;not null;default:0"`
		MerkleRoot          string         `gorm:"column:merkle_root;size:64;not null"`
		ActualFileSize      int64          `gorm:"column:actual_file_size;not null;default:0"`
		ActualFileHash      string         `gorm:"column:actual_file_hash;size:64;not null"`
		MimeType            string         `gorm:"column:mimetype;size:64;not null"`
		WriteMarker         string         `gorm:"column:write_marker;size:64;not null"`
		ThumbnailSize       int64          `gorm:"column:thumbnail_size;not null;default:0"`
		ThumbnailHash       string         `gorm:"column:thumbnail_hash;size:64;not null"`
		ActualThumbnailSize int64          `gorm:"column:actual_thumbnail_size;not null;default:0"`
		ActualThumbnailHash string         `gorm:"column:actual_thumbnail_hash;size:64;not null"`
		EncryptedKey        string         `gorm:"column:encrypted_key;size:64"`
		Attributes          datatypes.JSON `gorm:"column:attributes"`

		OnCloud        bool                      `gorm:"column:on_cloud"`
		CommitMetaTxns []reference.CommitMetaTxn `gorm:"foreignkey:ref_id"`
		CreatedAt      time.Time                 `gorm:"column:created_at;not null;default:now()"`
		UpdatedAt      time.Time                 `gorm:"column:updated_at;not null;default:now();index:idx_updated_at;"`

		ChunkSize int64 `gorm:"column:chunk_size;not null;default:65536"`
	}

	return new(Ref)
}
