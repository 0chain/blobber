package transaction

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/0chain/gosdk/zcncore"

	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
)

//Transaction entity that encapsulates the transaction related data and meta data
type Transaction struct {
	Hash              string           `json:"hash,omitempty"`
	Version           string           `json:"version,omitempty"`
	ClientID          string           `json:"client_id,omitempty"`
	PublicKey         string           `json:"public_key,omitempty"`
	ToClientID        string           `json:"to_client_id,omitempty"`
	ChainID           string           `json:"chain_id,omitempty"`
	TransactionData   string           `json:"transaction_data,omitempty"`
	Value             int64            `json:"transaction_value,omitempty"`
	Signature         string           `json:"signature,omitempty"`
	CreationDate      common.Timestamp `json:"creation_date,omitempty"`
	TransactionType   int              `json:"transaction_type,omitempty"`
	TransactionOutput string           `json:"transaction_output,omitempty"`
	OutputHash        string           `json:"txn_output_hash"`
	zcntxn            zcncore.TransactionScheme
	wg                *sync.WaitGroup
}

type SmartContractTxnData struct {
	Name      string      `json:"name"`
	InputArgs interface{} `json:"input"`
}

// Terms represents Blobber terms. A Blobber can update its terms,
// but any existing offer will use terms of offer signing time.
type Terms struct {
	// ReadPrice is price for reading. Token / GB.
	ReadPrice int64 `json:"read_price"`
	// WritePrice is price for reading. Token / GB. Also,
	// it used to calculate min_lock_demand value.
	WritePrice int64 `json:"write_price"`
	// MinLockDemand in number in [0; 1] range. It represents part of
	// allocation should be locked for the blobber rewards even if
	// user never write something to the blobber.
	MinLockDemand float64 `json:"min_lock_demand"`
	// MaxOfferDuration with this prices and the demand.
	MaxOfferDuration time.Duration `json:"max_offer_duration"`
	// ChallengeCompletionTime is duration required to complete a
	// challenge.
	ChallengeCompletionTime time.Duration `json:"challenge_completion_time"`
}

type StakePoolSettings struct {
	// DelegateWallet for pool owner.
	DelegateWallet string `json:"delegate_wallet"`
	// MinStake allowed.
	MinStake int64 `json:"min_stake"`
	// MaxStake allowed.
	MaxStake int64 `json:"max_stake"`
	// NumDelegates maximum allowed.
	NumDelegates int `json:"num_delegates"`
	// ServiceCharge of the blobber.
	ServiceCharge float64 `json:"service_charge"`
}

type StorageNodeGeolocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type StorageNode struct {
	ID                string                 `json:"id"`
	BaseURL           string                 `json:"url"`
	Geolocation       StorageNodeGeolocation `json:"geolocation"`
	Terms             Terms                  `json:"terms"`
	Capacity          int64                  `json:"capacity"`
	PublicKey         string                 `json:"-"`
	StakePoolSettings StakePoolSettings      `json:"stake_pool_settings"`
}

type BlobberAllocation struct {
	BlobberID string `json:"blobber_id"`
	Terms     Terms  `json:"terms"`
}

type StorageAllocation struct {
	ID             string               `json:"id"`
	Tx             string               `json:"tx"`
	OwnerPublicKey string               `json:"owner_public_key"`
	OwnerID        string               `json:"owner_id"`
	Size           int64                `json:"size"`
	UsedSize       int64                `json:"used_size"`
	Expiration     common.Timestamp     `json:"expiration_date"`
	CCT            time.Duration        `json:"challenge_completion_time"`
	TimeUnit       time.Duration        `json:"time_unit"`
	Blobbers       []*StorageNode       `json:"blobbers"`
	BlobberDetails []*BlobberAllocation `json:"blobber_details"`
	Finalized      bool                 `json:"finalized"`
	IsImmutable    bool                 `json:"is_immutable"`
}

func (sa *StorageAllocation) Until() common.Timestamp {
	return sa.Expiration + common.Timestamp(sa.CCT/time.Second)
}

type StorageAllocationBlobber struct {
	BlobberID      string `json:"blobber_id"`
	Size           int64  `json:"size"`
	UsedSize       int64  `json:"used_size"`
	AllocationRoot string `json:"allocation_root"`
}

const (
	ADD_BLOBBER_SC_NAME      = "add_blobber"
	ADD_VALIDATOR_SC_NAME    = "add_validator"
	CLOSE_CONNECTION_SC_NAME = "commit_connection"
	READ_REDEEM              = "read_redeem"
	CHALLENGE_RESPONSE       = "challenge_response"
	BLOBBER_HEALTH_CHECK     = "blobber_health_check"
	FINALIZE_ALLOCATION      = "finalize_allocation"
)

const STORAGE_CONTRACT_ADDRESS = "6dba10422e368813802877a85039d3985d96760ed844092319743fb3a76712d7"

func NewTransactionEntity() (*Transaction, error) {
	txn := &Transaction{}
	txn.Version = "1.0"
	txn.ClientID = node.Self.ID
	txn.CreationDate = common.Now()
	txn.ChainID = chain.GetServerChain().ID
	txn.PublicKey = node.Self.PublicKey
	txn.wg = &sync.WaitGroup{}
	zcntxn, err := zcncore.NewTransaction(txn, 0)
	if err != nil {
		return nil, err
	}
	txn.zcntxn = zcntxn
	return txn, nil
}

func (t *Transaction) ExecuteSmartContract(address, methodName, input string, val int64) error {
	t.wg.Add(1)
	err := t.zcntxn.ExecuteSmartContract(address, methodName, input, val)
	if err != nil {
		t.wg.Done()
		return err
	}
	t.wg.Wait()
	t.Hash = t.zcntxn.GetTransactionHash()
	if len(t.zcntxn.GetTransactionError()) > 0 {
		return common.NewError("transaction_send_error", t.zcntxn.GetTransactionError())
	}
	return nil
}

func (t *Transaction) Verify() error {
	if err := t.zcntxn.SetTransactionHash(t.Hash); err != nil {
		return err
	}
	t.wg.Add(1)
	err := t.zcntxn.Verify()
	if err != nil {
		t.wg.Done()
		return err
	}
	t.wg.Wait()
	if len(t.zcntxn.GetVerifyError()) > 0 {
		return common.NewError("transaction_verify_error", t.zcntxn.GetVerifyError())
	}
	output := t.zcntxn.GetVerifyOutput()

	var objmap map[string]json.RawMessage
	err = json.Unmarshal([]byte(output), &objmap)
	if err != nil {
		return common.NewError("transaction_verify_error", "Error unmarshaling verify output. "+err.Error())
	}

	err = json.Unmarshal(objmap["txn"], t)
	if err != nil {
		var confirmation map[string]json.RawMessage
		err = json.Unmarshal(objmap["confirmation"], &confirmation)
		if err != nil {
			return common.NewError("transaction_verify_error", "Error unmarshaling verify output. "+err.Error())
		}
		err = json.Unmarshal(confirmation["txn"], t)
		if err != nil {
			return common.NewError("transaction_verify_error", "Error unmarshaling verify output. "+err.Error())
		}
	}
	return nil
}

// func (t *Transaction) ComputeHashAndSign() error {
// 	hashdata := fmt.Sprintf("%v:%v:%v:%v:%v", t.CreationDate, t.ClientID,
// 		t.ToClientID, t.Value, encryption.Hash(t.TransactionData))
// 	t.Hash = encryption.Hash(hashdata)
// 	var err error
// 	t.Signature, err = node.Self.Sign(t.Hash)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

func (t *Transaction) OnTransactionComplete(zcntxn *zcncore.Transaction, status int) {
	t.wg.Done()
}

func (t *Transaction) OnVerifyComplete(zcntxn *zcncore.Transaction, status int) {
	t.wg.Done()
}

func (t *Transaction) OnAuthComplete(zcntxn *zcncore.Transaction, status int) {

}
