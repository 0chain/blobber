package challenge

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"

	"gorm.io/datatypes"
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
	ChallengeID             string                `gorm:"column:challenge_id;primary_key" json:"id"`
	PrevChallengeID         string                `gorm:"column:prev_challenge_id" json:"prev_id"`
	RandomNumber            int64                 `gorm:"column:seed" json:"seed"`
	AllocationID            string                `gorm:"column:allocation_id" json:"allocation_id"`
	AllocationRoot          string                `gorm:"column:allocation_root" json:"allocation_root"`
	RespondedAllocationRoot string                `gorm:"column:responded_allocation_root" json:"responded_allocation_root"`
	Status                  ChallengeStatus       `gorm:"column:status" json:"status"`
	Result                  ChallengeResult       `gorm:"column:result" json:"result"`
	StatusMessage           string                `gorm:"column:status_message" json:"status_message"`
	CommitTxnID             string                `gorm:"column:commit_txn_id" json:"commit_txn_id"`
	BlockNum                int64                 `gorm:"column:block_num" json:"block_num"`
	ValidatorsString        datatypes.JSON        `gorm:"column:validators" json:"-"`
	ValidationTicketsString datatypes.JSON        `gorm:"column:validation_tickets" json:"-"`
	LastCommitTxnList       datatypes.JSON        `gorm:"column:last_commit_txn_ids" json:"-"`
	RefID                   int64                 `gorm:"column:ref_id" json:"-"`
	Validators              []ValidationNode      `gorm:"-" json:"validators"`
	LastCommitTxnIDs        []string              `gorm:"-" json:"last_commit_txn_ids"`
	ValidationTickets       []*ValidationTicket   `gorm:"-" json:"validation_tickets"`
	ObjectPathString        datatypes.JSON        `gorm:"column:object_path" json:"-"`
	ObjectPath              *reference.ObjectPath `gorm:"-" json:"object_path"`
	Created                 common.Timestamp      `gorm:"-" json:"created"`

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

	db := datastore.GetStore().GetTransaction(ctx)
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

func GetLastChallengeEntity(ctx context.Context) (*ChallengeEntity, error) {
	db := datastore.GetStore().GetTransaction(ctx)
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
