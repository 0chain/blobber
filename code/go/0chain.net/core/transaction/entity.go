package transaction

import (
	"encoding/json"
	"github.com/0chain/gosdk/core/transaction"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"

	"github.com/0chain/gosdk/zcncore"

	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
)

var Last50Transactions []string

// Transaction entity that encapsulates the transaction related data and meta data
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

func NewTransactionEntity() (*Transaction, error) {
	txn := &Transaction{}
	txn.Version = "1.0"
	txn.ClientID = node.Self.ID
	txn.CreationDate = common.Now()
	txn.ChainID = chain.GetServerChain().ID
	txn.PublicKey = node.Self.PublicKey
	txn.wg = &sync.WaitGroup{}
	zcntxn, err := zcncore.NewTransaction(txn, 0, 0)
	if err != nil {
		return nil, err
	}
	txn.zcntxn = zcntxn
	return txn, nil
}

func (t *Transaction) GetTransaction() zcncore.TransactionScheme {
	return t.zcntxn
}

func (t *Transaction) ExecuteSmartContract(address, methodName string, input interface{}, val uint64) error {
	t.wg.Add(1)

	logging.Logger.Info("Jayash", zap.Any("address", address), zap.Any("methodName", methodName), zap.Any("input", input), zap.Any("val", val))

	sn := transaction.SmartContractTxnData{Name: methodName, InputArgs: input}
	snBytes, err := json.Marshal(sn)
	if err != nil {
		logging.Logger.Error("Jayash", zap.Error(err))
		return err
	}

	if len(Last50Transactions) == 50 {
		Last50Transactions = Last50Transactions[1:]
	} else {
		Last50Transactions = append(Last50Transactions, string(snBytes))
	}

	nonce := monitor.getNextUnusedNonce()
	if err := t.zcntxn.SetTransactionNonce(nonce); err != nil {
		logging.Logger.Error("Failed to set nonce.",
			zap.Any("hash", t.zcntxn.GetTransactionHash()),
			zap.Any("nonce", nonce),
			zap.Any("error", err))
	}

	logging.Logger.Info("Transaction nonce set.",
		zap.Any("hash", t.zcntxn.GetTransactionHash()),
		zap.Any("nonce", nonce))

	_, err = t.zcntxn.ExecuteSmartContract(address, methodName, input, uint64(val))
	if err != nil {
		t.wg.Done()
		logging.Logger.Error("Failed to execute SC.",
			zap.Any("hash", t.zcntxn.GetTransactionHash()),
			zap.Any("nonce", t.zcntxn.GetTransactionNonce()),
			zap.Any("error", err))
		monitor.recordFailedNonce(t.zcntxn.GetTransactionNonce())
		return err
	}

	t.wg.Wait()

	t.Hash = t.zcntxn.GetTransactionHash()
	if len(t.zcntxn.GetTransactionError()) > 0 {
		logging.Logger.Error("Failed to submit SC.",
			zap.Any("hash", t.zcntxn.GetTransactionHash()),
			zap.Any("nonce", t.zcntxn.GetTransactionNonce()),
			zap.Any("error", t.zcntxn.GetTransactionError()))
		monitor.recordFailedNonce(t.zcntxn.GetTransactionNonce())
		return common.NewError("transaction_send_error", t.zcntxn.GetTransactionError())
	}
	return nil
}

func (t *Transaction) Verify() error {
	if err := t.zcntxn.SetTransactionHash(t.Hash); err != nil {
		monitor.recordFailedNonce(t.zcntxn.GetTransactionNonce())
		logging.Logger.Error("Failed to set txn hash.",
			zap.Any("hash", t.zcntxn.GetTransactionHash()),
			zap.Any("nonce", t.zcntxn.GetTransactionNonce()),
			zap.Any("error", err))
		return err
	}
	t.wg.Add(1)
	err := t.zcntxn.Verify()
	if err != nil {
		t.wg.Done()
		logging.Logger.Error("Failed to start txn verification.",
			zap.Any("hash", t.zcntxn.GetTransactionHash()),
			zap.Any("nonce", t.zcntxn.GetTransactionNonce()),
			zap.Any("error", err))
		monitor.recordFailedNonce(t.zcntxn.GetTransactionNonce())
		return err
	}
	t.wg.Wait()
	if len(t.zcntxn.GetVerifyError()) > 0 {
		logging.Logger.Error("Failed to verify txn.",
			zap.Any("hash", t.zcntxn.GetTransactionHash()),
			zap.Any("nonce", t.zcntxn.GetTransactionNonce()),
			zap.Any("error", t.zcntxn.GetVerifyError()),
			zap.Any("verify_output", t.zcntxn.GetVerifyOutput()))
		monitor.recordFailedNonce(t.zcntxn.GetTransactionNonce())
		return common.NewError("transaction_verify_error", t.zcntxn.GetVerifyError())
	} else {
		logging.Logger.Info("Successful txn verification.",
			zap.Any("hash", t.zcntxn.GetTransactionHash()),
			zap.Any("nonce", t.zcntxn.GetTransactionNonce()))
		monitor.recordSuccess(t.zcntxn.GetTransactionNonce())
	}

	output := t.zcntxn.GetVerifyOutput()

	var objmap map[string]json.RawMessage
	err = json.Unmarshal([]byte(output), &objmap)
	if err != nil {
		// it is a plain error message from blockchain. The format is `error_code: error message`. eg verify_challenge: could not find challenge, value not present
		// so it is impossible to decode as map[string]json.RawMessage.
		return common.NewError("transaction_verify_error", string(output))
	}

	err = json.Unmarshal(objmap["txn"], t)
	if err != nil {
		var confirmation map[string]json.RawMessage
		err = json.Unmarshal(objmap["confirmation"], &confirmation)
		if err != nil {
			return common.NewError("transaction_verify_error", "Error unmarshaling verify output->confirmation: "+string(output)+" "+err.Error())
		}
		err = json.Unmarshal(confirmation["txn"], t)
		if err != nil {
			return common.NewError("transaction_verify_error", "Error unmarshaling verify output->confirmation->txn: "+string(output)+" "+err.Error())
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
