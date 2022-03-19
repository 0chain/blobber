//This file is used to create table schemas using gorm's automigration feature which takes information from
//struct's fields and functions

package automigration

import (
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/challenge"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"gorm.io/gorm"
)

//Create indexes
// ref: https://gorm.io/docs/indexes.html
// We can create unique index, multiple indexes, composite indexes with priority set

// // reference_objects table
// type ReferenceObjectsWithIndexes struct {
// 	reference.Ref
// 	Path         string    `gorm:"column:path;index:idx_path_alloc,priority:2;index:path_idx"`
// 	LookupHash   string    `gorm:"column:lookup_hash;index:idx_lookup_hash_alloc,priority:2"`
// 	AllocationID string    `gorm:"column:allocation_id;index:idx_path_alloc,priority:1;index:idx_lookup_hash_alloc,priority:1"`
// 	UpdatedAt    time.Time `gorm:"column:updated_at;index:idx_updated_at"`
// }

// // allocations table
// type AllocationsWithIndexes struct {
// 	allocation.Allocation
// 	Tx string `gorm:"column:tx;index:idx_unique_allocations_tx,unique"`
// }

// // pendings table
// type PendingsWithIndexes struct {
// 	allocation.Pending
// 	ClientID     string `gorm:"column:client_id;index:idx_pendings_cab,priority:1"`
// 	AllocationID string `gorm:"column:allocation_id;index:idx_pendings_cab,priority:2"`
// 	BlobberID    string `gorm:"column:blobber_id;index:idx_pendings_cab,priority:3"`
// }

// // read_pools table
// type ReadPoolsWithIndexes struct {
// 	allocation.ReadPool
// 	ClientID     string `gorm:"column:client_id;index:idx_read_pools_cab,priority:1"`
// 	AllocationID string `gorm:"column:allocation_id;index:idx_read_pools_cab,priority:2"`
// 	BlobberID    string `gorm:"column:blobber_id;index:idx_read_pools_cab,priority:3"`
// }

// // write_pools table
// type WritePoolsWithIndexes struct {
// 	allocation.WritePool
// 	ClientID     string `gorm:"column:client_id;index:idx_write_pools_cab,priority:1"`
// 	AllocationID string `gorm:"column:allocation_id;index:idx_write_pools_cab,priority:2"`
// 	BlobberID    string `gorm:"column:blobber_id;index:idx_write_pools_cab,priority:3"`
// }

// // marketplace_share_info table
// type ShareInfoWithIndexes struct {
// 	reference.ShareInfo
// 	OwnerID      string `gorm:"owner_id;index:idx_marketplace_share_info_for_owner,priority:1"`
// 	ClientID     string `gorm:"client_id;index:idx_marketplace_share_info_for_client,priority:1"`
// 	FilePathHash string `gorm:"file_path_hash;index:idx_marketplace_share_info_for_owner,priority:2;index:idx_marketplace_share_info_for_client,priority:2"`
// }

var tables = []interface{}{
	new(reference.Ref),
	new(reference.CommitMetaTxn),
	new(reference.ShareInfo),
	new(reference.Collaborator),
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

func MigrateSchema(db *gorm.DB) error {
	// Delete all tables, its indexes and constraints
	if err := db.Migrator().DropTable(tables...); err != nil {
		return err
	}

	// Migrate current table schema, indexes
	return db.AutoMigrate(tables...)
}
