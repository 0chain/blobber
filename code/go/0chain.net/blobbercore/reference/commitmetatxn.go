package reference

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"gorm.io/gorm"
)

// type CommitMetaTxn struct {
// 	RefID     int64     `gorm:"ref_id;not null" json:"ref_id"`
// 	TxnID     string    `gorm:"txn_id;size:64;not null" json:"txn_id"`
// 	CreatedAt time.Time `gorm:"created_at;not null" json:"created_at"`
// }

type CommitMetaTxn struct {
	RefID     int64     `gorm:"ref_id" json:"ref_id"`
	TxnID     string    `gorm:"txn_id" json:"txn_id"`
	CreatedAt time.Time `gorm:"created_at" json:"created_at"`
}

func (c *CommitMetaTxn) BeforeCreate(tx *gorm.DB) error {
	c.CreatedAt = time.Now()
	return nil
}

func (CommitMetaTxn) TableName() string {
	return "commit_meta_txns"
}

func AddCommitMetaTxn(ctx context.Context, refID int64, txnID string) error {
	db := datastore.GetStore().GetTransaction(ctx)
	return db.Create(&CommitMetaTxn{
		RefID: refID,
		TxnID: txnID,
	}).Error
}

func GetCommitMetaTxns(ctx context.Context, refID int64) ([]CommitMetaTxn, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	commitMetaTxns := []CommitMetaTxn{}
	err := db.Table((&CommitMetaTxn{}).TableName()).
		Where("ref_id = ?", refID).
		Order("created_at desc").
		Find(&commitMetaTxns).Error
	return commitMetaTxns, err
}
