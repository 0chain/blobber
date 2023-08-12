package stats

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"gorm.io/datatypes"
)

type ChallengeEntity struct {
	ChallengeID             string                `json:"id" gorm:"column:challenge_id;primaryKey"`
	PrevChallengeID         string                `json:"prev_id" gorm:"column:prev_challenge_id"`
	RandomNumber            int64                 `json:"seed" gorm:"column:seed"`
	AllocationID            string                `json:"allocation_id" gorm:"column:allocation_id"`
	AllocationRoot          string                `json:"allocation_root" gorm:"column:allocation_root"`
	RespondedAllocationRoot string                `json:"responded_allocation_root" gorm:"column:responded_allocation_root"`
	Status                  int                   `json:"status" gorm:"column:status"`
	Result                  int                   `json:"result" gorm:"column:result"`
	StatusMessage           string                `json:"status_message" gorm:"column:status_message"`
	CommitTxnID             string                `json:"commit_txn_id" gorm:"column:commit_txn_id"`
	BlockNum                int64                 `json:"block_num" gorm:"column:block_num"`
	ValidationTicketsString datatypes.JSON        `json:"-" gorm:"column:validation_tickets"`
	ValidatorsString        datatypes.JSON        `json:"-" gorm:"column:validators"`
	LastCommitTxnList       datatypes.JSON        `json:"-" gorm:"column:last_commit_txn_ids"`
	RefID                   int64                 `json:"-" gorm:"column:ref_id"`
	LastCommitTxnIDs        []string              `json:"last_commit_txn_ids" gorm:"-"`
	ObjectPathString        datatypes.JSON        `json:"-" gorm:"column:object_path"`
	ObjectPath              *reference.ObjectPath `json:"object_path" gorm:"-"`
	CreatedAt               time.Time             `gorm:"created_at"`
	UpdatedAt               time.Time             `gorm:"updated_at"`
}

func (ChallengeEntity) TableName() string {
	return "challenges"
}
func getAllFailedChallenges(ctx context.Context, offset, limit int) ([]ChallengeEntity, int, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	crs := []ChallengeEntity{}
	err := db.Offset(offset).Limit(limit).Order("challenge_id DESC").Table(ChallengeEntity{}.TableName()).Find(&crs, ChallengeEntity{Result: 2}).Error
	if err != nil {
		return nil, 0, err
	}

	var count int64
	err = db.Table(ChallengeEntity{}.TableName()).Where("result = ?", 2).Count(&count).Error
	if err != nil {
		return nil, 0, err
	}

	return crs, int(count), nil
}
