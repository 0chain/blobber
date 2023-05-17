package stats

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"gorm.io/gorm"
)

type FileStats struct {
	ID                       int64         `gorm:"column:id;primaryKey" json:"-"`
	RefID                    int64         `gorm:"column:ref_id;unique" json:"-"`
	Ref                      reference.Ref `gorm:"foreignKey:RefID;constraint:OnDelete:CASCADE"`
	NumUpdates               int64         `gorm:"column:num_of_updates" json:"num_of_updates"`
	NumBlockDownloads        int64         `gorm:"column:num_of_block_downloads" json:"num_of_block_downloads"`
	SuccessChallenges        int64         `gorm:"column:num_of_challenges" json:"num_of_challenges"`
	FailedChallenges         int64         `gorm:"column:num_of_failed_challenges" json:"num_of_failed_challenges"`
	LastChallengeResponseTxn string        `gorm:"column:last_challenge_txn;size:64" json:"last_challenge_txn"`
	WriteMarkerRedeemTxn     string        `gorm:"-" json:"write_marker_txn"`
	OnChain                  bool          `gorm:"-" json:"on_chain"`
	datastore.ModelWithTS
}

func (FileStats) TableName() string {
	return "file_stats"
}

func (f *FileStats) BeforeCreate(tx *gorm.DB) error {
	f.CreatedAt = time.Now()
	f.UpdatedAt = f.CreatedAt
	return nil
}

func (f *FileStats) BeforeSave(tx *gorm.DB) error {
	f.UpdatedAt = time.Now()
	return nil
}

func NewDirCreated(ctx context.Context, refID int64) error {
	db := datastore.GetStore().GetTransaction(ctx)
	stats := &FileStats{RefID: refID}
	stats.NumBlockDownloads = 0
	stats.NumUpdates = 1
	return db.Save(stats).Error
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
	db.Model(stats).Where(FileStats{RefID: refID}).Update("num_of_block_downloads", gorm.Expr("num_of_block_downloads + ?", 1))
}

func GetFileStats(ctx context.Context, refID int64) (*FileStats, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	stats := &FileStats{RefID: refID}
	err := db.Model(stats).Where(FileStats{RefID: refID}).Preload("Ref").First(stats).Error
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func DeleteFileStats(ctx context.Context, refID int64) error {
	db := datastore.GetStore().GetTransaction(ctx)
	return db.Unscoped().Delete(&FileStats{}, "ref_id=?", refID).Error
}
