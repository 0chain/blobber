package challenge

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/gosdk/constants"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type (
	ChallengeStatus int
	ChallengeResult int
)

const (
	Accepted ChallengeStatus = iota + 1
	Processed
	Committed
)

const (
	ChallengeUnknown ChallengeResult = iota
	ChallengeSuccess
	ChallengeFailure
)

type ValidationTicket struct {
	ChallengeID  string           `json:"challenge_id"`
	BlobberID    string           `json:"blobber_id"`
	ValidatorID  string           `json:"validator_id"`
	ValidatorKey string           `json:"validator_key"`
	Result       bool             `json:"success"`
	Message      string           `json:"message"`
	MessageCode  string           `json:"message_code"`
	Timestamp    common.Timestamp `json:"timestamp"`
	Signature    string           `json:"signature"`
}

func (vt *ValidationTicket) VerifySign() (bool, error) {
	hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v", vt.ChallengeID, vt.BlobberID, vt.ValidatorID, vt.ValidatorKey, vt.Result, vt.Timestamp)
	hash := encryption.Hash(hashData)
	verified, err := encryption.Verify(vt.ValidatorKey, vt.Signature, hash)
	return verified, err
}

type ValidationNode struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type ChallengeEntity struct {
	ChallengeID             string                `json:"id" gorm:"column:challenge_id;primary_key"`
	PrevChallengeID         string                `json:"prev_id" gorm:"column:prev_challenge_id"`
	RandomNumber            int64                 `json:"seed" gorm:"column:seed"`
	AllocationID            string                `json:"allocation_id" gorm:"column:allocation_id"`
	AllocationRoot          string                `json:"allocation_root" gorm:"column:allocation_root"`
	RespondedAllocationRoot string                `json:"responded_allocation_root" gorm:"column:responded_allocation_root"`
	Status                  ChallengeStatus       `json:"status" gorm:"column:status"`
	Result                  ChallengeResult       `json:"result" gorm:"column:result"`
	StatusMessage           string                `json:"status_message" gorm:"column:status_message"`
	CommitTxnID             string                `json:"commit_txn_id" gorm:"column:commit_txn_id"`
	BlockNum                int64                 `json:"block_num" gorm:"column:block_num"`
	ValidationTicketsString datatypes.JSON        `json:"-" gorm:"column:validation_tickets"`
	ValidatorsString        datatypes.JSON        `json:"-" gorm:"column:validators"`
	LastCommitTxnList       datatypes.JSON        `json:"-" gorm:"column:last_commit_txn_ids"`
	RefID                   int64                 `json:"-" gorm:"column:ref_id"`
	Validators              []ValidationNode      `json:"validators" gorm:"-"`
	LastCommitTxnIDs        []string              `json:"last_commit_txn_ids" gorm:"-"`
	ValidationTickets       []*ValidationTicket   `json:"validation_tickets" gorm:"-"`
	ObjectPathString        datatypes.JSON        `json:"-" gorm:"column:object_path"`
	ObjectPath              *reference.ObjectPath `json:"object_path" gorm:"-"`
	Created                 common.Timestamp      `json:"created" gorm:"-"`

	CreatedAt time.Time `gorm:"created_at"`
	UpdatedAt time.Time `gorm:"updated_at"`
}

func (ChallengeEntity) TableName() string {
	return "challenges"
}

func marshalField(obj interface{}, dest *datatypes.JSON) error {
	mbytes, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	(*dest) = datatypes.JSON(string(mbytes))
	return nil
}

func unMarshalField(stringObj datatypes.JSON, dest interface{}) error {
	retBytes, err := stringObj.Value()
	if err != nil {
		return err
	}
	if retBytes != nil {
		return json.Unmarshal(retBytes.([]byte), dest)
	}
	return nil
}

func (cr *ChallengeEntity) Save(ctx context.Context) error {
	db := datastore.GetStore().GetTransaction(ctx)
	return cr.SaveWith(db)
}

func (cr *ChallengeEntity) SaveWith(db *gorm.DB) error {
	err := marshalField(cr.Validators, &cr.ValidatorsString)
	if err != nil {
		return err
	}
	err = marshalField(cr.LastCommitTxnIDs, &cr.LastCommitTxnList)
	if err != nil {
		return err
	}
	err = marshalField(cr.ValidationTickets, &cr.ValidationTicketsString)
	if err != nil {
		return err
	}
	err = marshalField(cr.ObjectPath, &cr.ObjectPathString)
	if err != nil {
		return err
	}

	err = db.Save(cr).Error
	return err
}

func (cr *ChallengeEntity) UnmarshalFields() error {
	var err error

	cr.Validators = make([]ValidationNode, 0)
	err = unMarshalField(cr.ValidatorsString, &cr.Validators)
	if err != nil {
		return err
	}

	cr.LastCommitTxnIDs = make([]string, 0)
	err = unMarshalField(cr.LastCommitTxnList, &cr.LastCommitTxnIDs)
	if err != nil {
		return err
	}

	cr.ValidationTickets = make([]*ValidationTicket, 0)
	err = unMarshalField(cr.ValidationTicketsString, &cr.ValidationTickets)
	if err != nil {
		return err
	}

	cr.ObjectPath = &reference.ObjectPath{}
	err = unMarshalField(cr.ObjectPathString, cr.ObjectPath)
	if err != nil {
		return err
	}

	return nil
}

func GetChallengeEntity(ctx context.Context, challengeID string) (*ChallengeEntity, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	cr := &ChallengeEntity{}
	err := db.Where(ChallengeEntity{ChallengeID: challengeID}).Take(cr).Error
	if err != nil {
		return nil, err
	}
	err = cr.UnmarshalFields()
	if err != nil {
		return nil, err
	}
	return cr, nil
}

func getLastChallengeEntity(db *gorm.DB) (*ChallengeEntity, error) {
	if db == nil {
		return nil, constants.ErrInvalidParameter
	}
	cr := &ChallengeEntity{}
	err := db.Order("sequence desc").First(cr).Error
	if err != nil {
		return nil, err
	}
	err = cr.UnmarshalFields()
	if err != nil {
		return nil, err
	}
	return cr, nil
}

func getLastChallengeID(db *gorm.DB) (string, error) {
	if db == nil {
		return "", constants.ErrInvalidParameter
	}

	var challengeID string

	err := db.Raw("SELECT challenge_id FROM challenges ORDER BY sequence DESC LIMIT 1").Row().Scan(&challengeID)

	if err == nil || errors.Is(err, sql.ErrNoRows) {
		return challengeID, nil
	}

	return "", err
}

// Exists check challenge if exists in db
func Exists(db *gorm.DB, challengeID string) bool {

	var count int64
	db.Raw("SELECT 1 FROM challenges WHERE challenge_id=?", challengeID).Count(&count)

	return count > 0

}
