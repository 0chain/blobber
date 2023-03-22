package challenge

import (
	"fmt"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"gorm.io/gorm/clause"
)

type ChallengeTiming struct {
	// ChallengeID is the challenge ID generated on blockchain.
	ChallengeID string `gorm:"column:challenge_id;size:64;primaryKey" json:"id"`

	// CreatedAtChain is when generated on blockchain.
	CreatedAtChain common.Timestamp `gorm:"created_at_chain" json:"created_at_chain"`
	// CreatedAtBlobber is when synchronized and created at blobber.
	CreatedAtBlobber common.Timestamp `gorm:"created_at_blobber" json:"created_at_blobber"`
	// FileSize is size of file that was randomly selected for challenge
	FileSize int64 `gorm:"file_size" json:"file_size"`
	// ProofGenTime is the time taken in millisecond to generate challenge proof for the file
	ProofGenTime int64 `gorm:"proof_gen_time" json:"proof_gen_time"`
	// CompleteValidation is when all validation tickets are all received.
	CompleteValidation common.Timestamp `gorm:"complete_validation" json:"complete_validation"`
	// TxnSubmission is when challenge response is first sent to blockchain.
	TxnSubmission common.Timestamp `gorm:"txn_submission" json:"txn_submission"`
	// TxnVerification is when challenge response is verified on blockchain.
	TxnVerification common.Timestamp `gorm:"txn_verification" json:"txn_verification"`
	// Expiration is when challenge is marked as expired by blobber.
	Expiration common.Timestamp `gorm:"expiration" json:"expiration"`
	// RetriesInChain is number of times challenge submission and verification request is sent to blockchain
	RetriesInChain int `gorm:"retries_in_chain" json:"retries_in_chain"`
	// ClosedAt is duration in seconds a challenge is closed (eg. expired, cancelled, or completed/verified)
	// after challenge Creation Date
	ClosedAt common.Timestamp `gorm:"column:closed_at;index:idx_closed_at,sort:desc;" json:"closed"`
}

func (ChallengeTiming) TableName() string {
	return "challenge_timing"
}

func (ct *ChallengeTiming) Save() error {
	db := datastore.GetStore().GetDB()
	return db.Save(ct).Error
}

func GetChallengeTimings(from common.Timestamp, limit common.Pagination) ([]*ChallengeTiming, error) {
	query := datastore.GetStore().GetDB().Model(&ChallengeTiming{}).
		Where("closed_at > ?", from).Limit(limit.Limit).Offset(limit.Offset).Order(clause.OrderByColumn{
		Column: clause.Column{Name: "closed_at"},
		Desc:   limit.IsDescending,
	})

	var chs []*ChallengeTiming

	result := query.Find(&chs)
	if result.Error != nil {
		return nil, fmt.Errorf("error retrieving updated challenge timings with %v; error: %v",
			from, result.Error)
	}
	return chs, nil
}
