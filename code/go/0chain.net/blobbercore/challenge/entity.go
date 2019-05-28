package challenge

import (
	"context"
	"encoding/json"
	"fmt"

	"0chain.net/blobbercore/datastore"
	"0chain.net/blobbercore/reference"
	"0chain.net/core/common"
	"0chain.net/core/encryption"

	"github.com/jinzhu/gorm/dialects/postgres"
)

type ChallengeStatus int
type ChallengeResult int

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
	ValidationTicketsString postgres.Jsonb        `json:"-" gorm:"column:validation_tickets"`
	ValidatorsString        postgres.Jsonb        `json:"-" gorm:"column:validators"`
	LastCommitTxnList       postgres.Jsonb        `json:"-" gorm:"column:last_commit_txn_ids"`
	Validators              []ValidationNode      `json:"validators" gorm:"-"`
	LastCommitTxnIDs        []string              `json:"last_commit_txn_ids" gorm:"-"`
	ValidationTickets       []*ValidationTicket   `json:"validation_tickets" gorm:"-"`
	ObjectPathString        postgres.Jsonb        `json:"-" gorm:"column:object_path"`
	ObjectPath              *reference.ObjectPath `json:"object_path" gorm:"-"`
}

func (ChallengeEntity) TableName() string {
	return "challenges"
}

func marshalField(obj interface{}, dest *postgres.Jsonb) error {
	mbytes, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	(*dest).RawMessage = json.RawMessage(string(mbytes))
	return nil
}

func unMarshalField(stringObj postgres.Jsonb, dest interface{}) error {
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
	err := db.Where(ChallengeEntity{ChallengeID: challengeID}).Find(cr).Error
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
