package stats

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/pkg/errors"

	"gorm.io/gorm"
)

type FileStats struct {
	ID                       int64  `gorm:"column:id;primary_key" json:"-"`
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

func NewDirCreated(ctx context.Context, refID int64) {
	logging.Logger.Info("NewDirCreated inner...")
	db := datastore.GetStore().GetTransaction(ctx)
	stats := &FileStats{RefID: refID}
	stats.NumBlockDownloads = 0
	stats.NumUpdates = 1
	db.Save(stats)
}

func NewFileCreated(ctx context.Context, refID int64) {
	logging.Logger.Info("NewFileCreated inner...")
	db := datastore.GetStore().GetTransaction(ctx)
	stats := &FileStats{RefID: refID}
	stats.NumBlockDownloads = 0
	stats.NumUpdates = 1
	db.Save(stats)
}

func FileUpdated(ctx context.Context, refID int64) {
	logging.Logger.Info("FileUpdated inner...")
	db := datastore.GetStore().GetTransaction(ctx)
	stats := &FileStats{RefID: refID}
	db.Model(stats).Where(stats).Update("num_of_updates", gorm.Expr("num_of_updates + ?", 1))
}

func FileBlockDownloaded(ctx context.Context, refID int64) error {
	logging.Logger.Info("FileBlockDownloaded inner...")
	db := datastore.GetStore().GetTransaction(ctx)
	//stats := &FileStats{RefID: refID}
	err := db.Model(&FileStats{}).Where("ref_id = ?", refID).
		Update("num_of_block_downloads", gorm.Expr("num_of_block_downloads + ?", 1)).
		Error
	if err != nil {
		return errors.Wrap(err, "db get error for FileBlockDownloaded")
	}
	return nil
}

func GetFileStats(ctx context.Context, refID int64) (*FileStats, error) {
	logging.Logger.Info("GetFileStats inner...")
	db := datastore.GetStore().GetTransaction(ctx)
	stats := &FileStats{RefID: refID}
	err := db.Model(stats).Where(stats).First(stats).Error
	if err != nil {
		return nil, errors.Wrap(err, "db get error")
	}
	return stats, err
}
