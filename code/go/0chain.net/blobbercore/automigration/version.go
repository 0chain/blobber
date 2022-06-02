package automigration

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
)

type Version struct {
	Version string `gorm:"column:version;size:8;not null"`
}

func (Version) TableName() string {
	return "version"
}

func AddVersion(v string) (err error) {
	ctx := datastore.GetStore().CreateTransaction(context.Background())
	db := datastore.GetStore().GetTransaction(ctx)

	defer func() {
		if err != nil {
			db.Rollback()
			return
		}
		db.Commit()

	}()
	version := &Version{Version: v}
	err = db.Model(&Version{}).Create(version).Error
	return
}
