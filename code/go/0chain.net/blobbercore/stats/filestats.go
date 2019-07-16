package stats

import (
	"context"

	"0chain.net/blobbercore/datastore"

	"github.com/jinzhu/gorm"
)

type FileStats struct {
	ID                       int64  `gorm:column:id;primary_key json:"-"`
	RefID                    int64  `gorm:"column:ref_id" json:"-"`
	NumUpdates               int64  `gorm:"column:num_of_updates" json:"num_of_updates"`
	NumBlockDownloads        int64  `gorm:"column:num_of_block_downloads" json:"num_of_block_downloads"`
	SuccessChallenges        int64  `gorm:"column:num_of_challenges" json:"num_of_challenges"`
	FailedChallenges         int64  `gorm:"column:num_of_failed_challenges" json:"num_of_failed_challenges"`
	LastChallengeResponseTxn string `gorm:"column:last_challenge_txn" json:"last_challenge_txn"`
	WriteMarkerRedeemTxn     string `gorm:"-" json:"write_marker_txn"`
	datastore.ModelWithTS

	//NumBlockWrites           int64  `gorm:"column:num_of_block_writes" json:"num_of_block_writes"`
}

func (FileStats) TableName() string {
	return "file_stats"
}

func NewFileCreated(ctx context.Context, refID int64) {
	db := datastore.GetStore().GetTransaction(ctx)
	stats := &FileStats{RefID: refID}
	stats.NumBlockDownloads = 0
	stats.NumUpdates = 1
	db.Save(stats)
}

func FileUpdated(ctx context.Context, refID int64) {
	db := datastore.GetStore().GetTransaction(ctx)
	stats := &FileStats{RefID: refID}
	db.Model(stats).Where(stats).Update("num_of_updates", gorm.Expr("num_of_updates + ?", 1))
}

func FileBlockDownloaded(ctx context.Context, refID int64) {
	db := datastore.GetStore().GetTransaction(ctx)
	stats := &FileStats{RefID: refID}
	db.Debug().Model(stats).Where(FileStats{RefID: refID}).Update("num_of_block_downloads", gorm.Expr("num_of_block_downloads + ?", 1))
}

func GetFileStats(ctx context.Context, refID int64) (*FileStats, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	stats := &FileStats{RefID: refID}
	err := db.Debug().Model(stats).Where(FileStats{RefID: refID}).First(stats).Error
	if err != nil {
		return nil, err
	}
	return stats, err
}
