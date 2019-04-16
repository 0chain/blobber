package challenge

import (
	"context"
	"fmt"

	"0chain.net/common"
	"0chain.net/datastore"
	"0chain.net/encryption"
	"0chain.net/lock"
	"0chain.net/reference"
	"0chain.net/transaction"
)

type ChallengeStatus int
type ChallengeResult int

const (
	Accepted  ChallengeStatus = 0
	Committed ChallengeStatus = 1
	Failed    ChallengeStatus = 2
	Error     ChallengeStatus = 3

	ChallengeSuccess ChallengeResult = 1
	ChallengeFailure ChallengeResult = 2
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
	Version           string                  `json:"version"`
	CreationDate      common.Timestamp        `json:"created"`
	ID                string                  `json:"id"`
	Validators        []ValidationNode        `json:"validators"`
	RandomNumber      int64                   `json:"seed"`
	AllocationID      string                  `json:"allocation_id"`
	Blobber           transaction.StorageNode `json:"blobber"`
	AllocationRoot    string                  `json:"allocation_root"`
	Status            ChallengeStatus         `json:"status"`
	Result            ChallengeResult         `json:"result"`
	StatusMessage     string                  `json:"status_message"`
	CommitTxnID       string                  `json:"commit_txn_id"`
	LastCommitTxnIDs  []string                `json:"last_commit_txn_ids"`
	BlockNum          int64                   `json:"block_num"`
	Retries           int                     `json:"retries"`
	WriteMarker       string                  `json:"write_marker"`
	ValidationTickets []*ValidationTicket     `json:"validation_tickets"`
	ObjectPath        *reference.ObjectPath   `json:"object_path"`
}

var challengeEntityMetaData *datastore.EntityMetadataImpl

/*Provider - entity provider for client object */
func Provider() datastore.Entity {
	t := &ChallengeEntity{}
	t.Version = "1.0"
	t.CreationDate = common.Now()
	t.LastCommitTxnIDs = make([]string, 0)
	return t
}

func SetupEntity(store datastore.Store) {
	challengeEntityMetaData = datastore.MetadataProvider()
	challengeEntityMetaData.Name = "challenge"
	challengeEntityMetaData.DB = "challenge"
	challengeEntityMetaData.Provider = Provider
	challengeEntityMetaData.Store = store

	datastore.RegisterEntityMetadata("challenge", challengeEntityMetaData)
}

func (ch *ChallengeEntity) GetEntityMetadata() datastore.EntityMetadata {
	return challengeEntityMetaData
}
func (ch *ChallengeEntity) SetKey(key datastore.Key) {
	ch.ID = key
}
func (ch *ChallengeEntity) GetKey() datastore.Key {
	return datastore.ToKey(challengeEntityMetaData.GetDBName() + ":" + ch.ID)
}
func (ch *ChallengeEntity) Read(ctx context.Context, key datastore.Key) error {
	return challengeEntityMetaData.GetStore().Read(ctx, key, ch)
}
func (ch *ChallengeEntity) Write(ctx context.Context) error {
	return challengeEntityMetaData.GetStore().Write(ctx, ch)
}
func (ch *ChallengeEntity) Delete(ctx context.Context) error {
	return nil
}

func RetakeChallenge(ctx context.Context, challengeID string) error {
	challengeObj := Provider().(*ChallengeEntity)
	challengeObj.ID = challengeID
	mutex := lock.GetMutex(challengeObj.GetKey())
	mutex.Lock()
	defer mutex.Unlock()
	ctx = challengeEntityMetaData.GetStore().WithConnection(ctx)
	defer challengeEntityMetaData.GetStore().Discard(ctx)

	err := challengeObj.Read(ctx, challengeObj.GetKey())
	if err != nil {
		return err
	}
	challengeObj.CommitTxnID = ""
	challengeObj.Retries = 0
	challengeObj.Status = Accepted
	challengeObj.ValidationTickets = make([]*ValidationTicket, len(challengeObj.Validators))
	err = challengeObj.Write(ctx)
	if err != nil {
		return err
	}
	err = challengeEntityMetaData.GetStore().Commit(ctx)
	return err
}
