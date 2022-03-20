//This file is used to create table schemas using gorm's automigration feature which takes information from
//struct's fields and functions

package automigration

import (
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/challenge"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
)

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

func MigrateSchema() error {
	db := datastore.GetStore().GetDB()

	// Delete all tables, its indexes and constraints
	if err := db.Migrator().DropTable(tables...); err != nil {
		return err
	}

	// Migrate current table schema, indexes
	return db.AutoMigrate(tables...)
}
