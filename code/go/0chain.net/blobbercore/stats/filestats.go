package stats

import (
	"context"

	"0chain.net/blobbercore/challenge"

	"0chain.net/blobbercore/datastore"

	"github.com/jinzhu/gorm"
)

type FileStats struct {
	ID                       int64  `gorm:column:id;primary_key`
	RefID                    int64  `gorm:"column:ref_id"`
	AllocationID             string `gorm:"column:allocation_id" json:"allocation_id"`
	NumUpdates               int64  `gorm:"column:num_of_updates" json:"num_of_updates"`
	NumBlockDownloads        int64  `gorm:"column:num_of_block_downloads" json:"num_of_block_downloads"`
	SuccessChallenges        int64  `gorm:"column:num_of_challenges" json:"num_of_challenges"`
	FailedChallenges         int64  `gorm:"column:num_of_failed_challenges" json:"num_of_failed_challenges"`
	LastChallengeResponseTxn string `gorm:"column:last_challenge_txn" json:"last_challenge_txn"`
	WriteMarkerRedeemTxn     string `gorm:"-" json:"write_marker_txn"`
}

func (FileStats) TableName() string {
	return "file_stats"
}

func FileUpdated(ctx context.Context, refID int64) {
	db := datastore.GetStore().GetTransaction(ctx)
	stats := &FileStats{RefID: refID}
	db.Where(stats).Update("num_of_updates", gorm.Expr("num_of_updates + ?", 1))
}

func FileBlockDownloaded(ctx context.Context, refID int64) {
	db := datastore.GetStore().GetTransaction(ctx)
	stats := &FileStats{RefID: refID}
	db.Where(stats).Update("num_of_block_downloads", gorm.Expr("num_of_block_downloads + ?", 1))
}

func FileChallenged(ctx context.Context, refID int64, result challenge.ChallengeResult, challengeTxn string) {
	db := datastore.GetStore().GetTransaction(ctx)
	stats := &FileStats{RefID: refID}
	if result == challenge.ChallengeSuccess {
		db.Where(stats).Updates(map[string]interface{}{"num_of_challenges": gorm.Expr("num_of_challenges + ?", 1), "last_challenge_txn": challengeTxn})
	} else if result == challenge.ChallengeFailure {
		db.Where(stats).Updates(map[string]interface{}{"num_of_failed_challenges": gorm.Expr("num_of_failed_challenges + ?", 1), "last_challenge_txn": challengeTxn})
	}
}
