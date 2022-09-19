package challenge

import (
	"fmt"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChallengeTiming struct {
	ChallengeID string `gorm:"column:challenge_id;size:64;primaryKey" json:"id"`

	// When generated on blockchain.
	CreatedAtChain common.Timestamp `gorm:"created_at_chain" json:"created_at_chain"`
	// When synchronized and created at blobber.
	CreatedAtBlobber common.Timestamp `gorm:"created_at_blobber" json:"created_at_blobber"`
	// When all validation tickets are all received.
	CompleteValidation common.Timestamp `gorm:"complete_validation" json:"complete_validation"`
	// When challenge response is first sent to blockchain.
	TxnSubmission common.Timestamp `gorm:"txn_submission" json:"txn_submission"`
	// When challenge response is verified on blockchain.
	TxnVerified common.Timestamp `gorm:"txn_verified" json:"txn_verification"`
	// When challenge is cancelled by blobber due to expiration or bad challenge data (eg. invalid ref or not a file) which is impossible to validate.
	Cancelled common.Timestamp `gorm:"cancelled" json:"cancelled"`
	// When challenge is marked as expired by blobber.
	Expiration common.Timestamp `gorm:"expiration" json:"expiration"`

	// When row is last updated
	UpdatedAt common.Timestamp `gorm:"updated_at" json:"updated"`
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
		err := tx.Model(&c).Update("cancellation", cancellation).Error

		if err == nil && reason == ErrExpiredCCT {
			err = tx.Model(&c).Update("expiration", cancellation).Error
		}

		return err
	})

	return err
}

func UpdateChallengeTimingFirstValidation(challengeID string, firstValidation common.Timestamp) error {
	c := &ChallengeTiming{
		ChallengeID: challengeID,
	}

	err := datastore.GetStore().GetDB().Transaction(func(tx *gorm.DB) error {
		return tx.Model(&c).Update("first_validation", firstValidation).Error
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
		return tx.Model(&c).Update("txn_verification", txnVerification).Error
	})

	return err
}

func GetUpdatedChallengeTimings(from common.Timestamp, limit common.Pagination) ([]*ChallengeTiming, error) {
	query := datastore.GetStore().GetDB().Model(&ChallengeTiming{}).
		Where("updated_at > ?", from).Limit(limit.Limit).Offset(limit.Offset).Order(clause.OrderByColumn{
		Column: clause.Column{Name: "created_at"},
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
