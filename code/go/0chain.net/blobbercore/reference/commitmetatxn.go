package reference

import (
	"context"

	"0chain.net/blobbercore/datastore"
)

type CommitMetaTxn struct {
	RefID int64  `gorm:"ref_id" json:"ref_id"`
	TxnID string `gorm:"txn_id" json:"txn_id"`
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
		Find(&commitMetaTxns).Error
	return commitMetaTxns, err
}
