package transaction

import (
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

var (
	Last50Transactions      []string
	last50TransactionsMutex sync.Mutex
)

type SmartContractTxnData struct {
	Name      string      `json:"name"`
	InputArgs interface{} `json:"input"`
}

// Terms represents Blobber terms. A Blobber can update its terms,
// but any existing offer will use terms of offer signing time.
type Terms struct {
	// ReadPrice is price for reading. Token / GB.
	ReadPrice uint64 `json:"read_price"`
	// WritePrice is price for reading. Token / GB. Also,
	// it used to calculate min_lock_demand value.
	WritePrice uint64 `json:"write_price"`
}

type StakePoolSettings struct {
	// DelegateWallet for pool owner.
	DelegateWallet string `json:"delegate_wallet"`
	// NumDelegates maximum allowed.
	NumDelegates int `json:"num_delegates"`
	// ServiceCharge of the blobber.
	ServiceCharge float64 `json:"service_charge"`
}

type StorageNode struct {
	ID                string            `json:"id"`
	BaseURL           string            `json:"url"`
	Terms             Terms             `json:"terms"`
	Capacity          int64             `json:"capacity"`
	PublicKey         string            `json:"-"`
	StakePoolSettings StakePoolSettings `json:"stake_pool_settings"`
	IsEnterprise      bool              `json:"is_enterprise"`
	StorageVersion    int               `json:"storage_version"`
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
	BlobberDetails []*BlobberAllocation `json:"blobber_details"`
	Finalized      bool                 `json:"finalized"`
	TimeUnit       time.Duration        `json:"time_unit"`
	WritePool      uint64               `json:"write_pool"`
	FileOptions    uint16               `json:"file_options"`
	StartTime      common.Timestamp     `json:"start_time"`
	StorageVersion int                  `json:"storage_version"`

	DataShards   int64 `json:"data_shards"`
	ParityShards int64 `json:"parity_shards"`
}

type StorageAllocationBlobber struct {
	BlobberID      string `json:"blobber_id"`
	Size           int64  `json:"size"`
	UsedSize       int64  `json:"used_size"`
	AllocationRoot string `json:"allocation_root"`
}

const (
	ADD_BLOBBER_SC_NAME      = "add_blobber"
	UPDATE_BLOBBER_SC_NAME   = "update_blobber_settings"
	ADD_VALIDATOR_SC_NAME    = "add_validator"
	CLOSE_CONNECTION_SC_NAME = "commit_connection"
	READ_REDEEM              = "read_redeem"
	CHALLENGE_RESPONSE       = "challenge_response"
	BLOBBER_HEALTH_CHECK     = "blobber_health_check"
	FINALIZE_ALLOCATION      = "finalize_allocation"
	VALIDATOR_HEALTH_CHECK   = "validator_health_check"
)

const STORAGE_CONTRACT_ADDRESS = "6dba10422e368813802877a85039d3985d96760ed844092319743fb3a76712d7"

func updateLast50Transactions(data string) {
	last50TransactionsMutex.Lock()
	defer last50TransactionsMutex.Unlock()

	if len(Last50Transactions) == 50 {
		Last50Transactions = Last50Transactions[1:]
	} else {
		Last50Transactions = append(Last50Transactions, data)
	}
}
