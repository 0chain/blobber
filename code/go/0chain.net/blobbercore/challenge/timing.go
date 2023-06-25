package challenge

import (
	"fmt"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"gorm.io/gorm"
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
	// Cancelled is when challenge is cancelled by blobber due to expiration or bad challenge data (eg. invalid ref or not a file) which is impossible to validate.
	Cancelled common.Timestamp `gorm:"cancelled" json:"cancelled"`
	// Expiration is when challenge is marked as expired by blobber.
	Expiration common.Timestamp `gorm:"expiration" json:"expiration"`

	// ClosedAt is when challenge is closed (eg. expired, cancelled, or completed/verified).
	ClosedAt common.Timestamp `gorm:"column:closed_at;index:idx_closed_at,sort:desc;" json:"closed"`

	// UpdatedAt is when row is last updated.
	UpdatedAt common.Timestamp `gorm:"column:updated_at;index:idx_updated_at,sort:desc;" json:"updated"`
}

func (ChallengeTiming) TableName() string {
	return "challenge_timing"
}

func (c *ChallengeTiming) BeforeCreate(tx *gorm.DB) error {
	c.UpdatedAt = common.Now()
	c.CreatedAtBlobber = common.Now()
	return nil
}

func (c *ChallengeTiming) BeforeSave(tx *gorm.DB) error {
	c.UpdatedAt = common.Now()
	return nil
}

func CreateChallengeTiming(challengeID string, createdAt common.Timestamp) error {
	c := &ChallengeTiming{
		ChallengeID:    challengeID,
		CreatedAtChain: createdAt,
	}

	err := datastore.GetStore().GetDB().Transaction(func(tx *gorm.DB) error {
		return tx.Create(c).Error
	})

	return err
}

func UpdateChallengeTimingCancellation(challengeID string, cancellation common.Timestamp, reason error) error {
	c := &ChallengeTiming{
		ChallengeID: challengeID,
	}

	err := datastore.GetStore().GetDB().Transaction(func(tx *gorm.DB) error {
		values := map[string]interface{}{
			"closed_at": cancellation,
		}

		if reason == ErrExpiredCCT {
			values["expiration"] = cancellation
		}

		return tx.Model(&c).Updates(values).Error
	})

	return err
}

func UpdateChallengeTimingCompleteValidation(challengeID string, completeValidation common.Timestamp) error {
	c := &ChallengeTiming{
		ChallengeID: challengeID,
	}

	err := datastore.GetStore().GetDB().Transaction(func(tx *gorm.DB) error {
		return tx.Model(&c).Update("complete_validation", completeValidation).Error
	})

	return err
}

func UpdateChallengeTimingProofGenerationAndFileSize(
	challengeID string, proofGenTime, size int64) error {

	if proofGenTime == 0 || size == 0 {
		logging.Logger.Error(fmt.Sprintf("Proof gen time: %d, size: %d", proofGenTime, size))
	}

	c := &ChallengeTiming{
		ChallengeID: challengeID,
	}

	err := datastore.GetStore().GetDB().Transaction(func(tx *gorm.DB) error {
		values := map[string]interface{}{
			"proof_gen_time": proofGenTime,
			"file_size":      size,
		}
		return tx.Model(&c).Updates(values).Error
	})

	return err
}

func UpdateChallengeTimingTxnSubmission(challengeID string, txnSubmission common.Timestamp) error {
	c := &ChallengeTiming{
		ChallengeID: challengeID,
	}

	err := datastore.GetStore().GetDB().Transaction(func(tx *gorm.DB) error {
		return tx.Model(&c).Update("txn_submission", txnSubmission).Error
	})

	return err
}

func UpdateChallengeTimingTxnVerification(challengeID string, txnVerification common.Timestamp) error {
	c := &ChallengeTiming{
		ChallengeID: challengeID,
	}

	err := datastore.GetStore().GetDB().Transaction(func(tx *gorm.DB) error {
		values := map[string]interface{}{
			"txn_verification": txnVerification,
			"closed_at":        txnVerification,
		}

		return tx.Model(&c).Updates(values).Error
	})

	return err
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

func GetChallengeTiming(challengeID string) (*ChallengeTiming, error) {
	var ch *ChallengeTiming

	err := datastore.GetStore().GetDB().Model(&ChallengeTiming{}).Where("challenge_id = ?", challengeID).First(&ch).Error
	return ch, err
}
