package challenge

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"

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
	Cancelled
)

func (s ChallengeStatus) String() string {
	switch s {
	case Accepted:
		return "Accepted"
	case Processed:
		return "Processed"
	case Committed:
		return "Committed"
	case Cancelled:
		return "Cancelled"
	default:
		return fmt.Sprintf("%d", int(s))
	}
}

const (
	ChallengeUnknown ChallengeResult = iota
	ChallengeSuccess
	ChallengeFailure
)

const (
	cleanupInterval = 30 * time.Minute
	cleanupGap      = 1 * time.Hour
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
	ChallengeID             string                `gorm:"column:challenge_id;size:64;primaryKey" json:"id"`
	PrevChallengeID         string                `gorm:"column:prev_challenge_id;size:64" json:"prev_id"`
	RandomNumber            int64                 `gorm:"column:seed;not null;default:0" json:"seed"`
	AllocationID            string                `gorm:"column:allocation_id;size64;not null" json:"allocation_id"`
	AllocationRoot          string                `gorm:"column:allocation_root;size:64" json:"allocation_root"`
	RespondedAllocationRoot string                `gorm:"column:responded_allocation_root;size:64" json:"responded_allocation_root"`
	Status                  ChallengeStatus       `gorm:"column:status;type:integer;not null;default:0;index:idx_status" json:"status"`
	Result                  ChallengeResult       `gorm:"column:result;type:integer;not null;default:0" json:"result"`
	StatusMessage           string                `gorm:"column:status_message" json:"status_message"`
	CommitTxnID             string                `gorm:"column:commit_txn_id;size:64" json:"commit_txn_id"`
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
	Sequence                int64                 `gorm:"column:sequence;unique;autoIncrement;<-:false"`
	Timestamp               common.Timestamp      `gorm:"column:timestamp;not null;default:0" json:"timestamp"`

	// This time is taken from Blockchain challenge object.
	RoundCreatedAt int64            `gorm:"round_created_at" json:"round_created_at"`
	CreatedAt      common.Timestamp `gorm:"created_at" json:"created"`
	UpdatedAt      time.Time        `gorm:"updated_at;type:timestamp without time zone;not null;default:current_timestamp" json:"-"`
	statusMutex    *sync.Mutex      `gorm:"-" json:"-"`
}

func (ChallengeEntity) TableName() string {
	return "challenges"
}

func (c *ChallengeEntity) BeforeCreate(tx *gorm.DB) error {
	c.UpdatedAt = time.Now()
	return nil
}

func (c *ChallengeEntity) BeforeSave(tx *gorm.DB) error {
	c.UpdatedAt = time.Now()
	return nil
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
	if len(stringObj) == 0 {
		return nil
	}
	retBytes, err := stringObj.MarshalJSON()
	if err != nil {
		return err
	}
	if retBytes != nil {
		return json.Unmarshal(retBytes, dest)
	}
	return nil
}

func (cr *ChallengeEntity) Save(ctx context.Context) error {
	db := datastore.GetStore().GetTransaction(ctx)
	return cr.SaveWith(db.DB)
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
	err = unMarshalField(cr.ValidatorsString, &cr.Validators)
	if err != nil {
		return err
	}

	err = unMarshalField(cr.LastCommitTxnList, &cr.LastCommitTxnIDs)
	if err != nil {
		return err
	}

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

func SetupChallengeCleanUpWorker(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(cleanupInterval):
				cleanUpWorker()
			}
		}
	}()
}

func cleanUpWorker() {
	currentRound := roundInfo.CurrentRound + int64(float64(roundInfo.LastRoundDiff)*(float64(time.Since(roundInfo.CurrentRoundCaptureTime).Milliseconds())/float64(GetRoundInterval.Milliseconds())))
	_ = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		db := datastore.GetStore().GetTransaction(ctx)
		return db.Model(&ChallengeEntity{}).Unscoped().Delete(&ChallengeEntity{}, "status <> ? AND round_created_at < ?", Cancelled, currentRound-config.Configuration.ChallengeCleanupGap).Error
	})
}
