package transaction

import (
	"fmt"

	"0chain.net/chain"
	"0chain.net/common"
	"0chain.net/encryption"
	"0chain.net/node"
)

//Transaction entity that encapsulates the transaction related data and meta data
type Transaction struct {
	Hash            string           `json:"hash,omitempty"`
	Version         string           `json:"version,omitempty"`
	ClientID        string           `json:"client_id,omitempty"`
	PublicKey       string           `json:"public_key,omitempty"`
	ToClientID      string           `json:"to_client_id,omitempty"`
	ChainID         string           `json:"chain_id,omitempty"`
	TransactionData string           `json:"transaction_data,omitempty"`
	Value           int64            `json:"transaction_value,omitempty"`
	Signature       string           `json:"signature,omitempty"`
	CreationDate    common.Timestamp `json:"creation_date,omitempty"`
	TransactionType int              `json:"transaction_type,omitempty"`
}

type SmartContractTxnData struct {
	Name      string      `json:"name"`
	InputArgs interface{} `json:"input"`
}

type StorageNode struct {
	ID        string `json:"id"`
	BaseURL   string `json:"url"`
	PublicKey string `json:"-"`
}

const ADD_BLOBBER_SC_NAME = "add_blobber"
const CLOSE_CONNECTION_SC_NAME = "close_connection"
const TXN_SUBMIT_URL = "v1/transaction/put"
const STORAGE_CONTRACT_ADDRESS = "6dba10422e368813802877a85039d3985d96760ed844092319743fb3a76712d7"
const MAX_TXN_RETRIES = 3

func NewTransactionEntity() *Transaction {
	txn := &Transaction{}
	txn.Version = "1.0"
	txn.ClientID = node.Self.ID
	txn.CreationDate = common.Now()
	txn.ChainID = chain.GetServerChain().ID
	txn.PublicKey = node.Self.PublicKey
	return txn
}

func (t *Transaction) ComputeHashAndSign() error {
	hashdata := fmt.Sprintf("%v:%v:%v:%v:%v", t.CreationDate, t.ClientID,
		t.ToClientID, t.Value, encryption.Hash(t.TransactionData))
	t.Hash = encryption.Hash(hashdata)
	var err error
	t.Signature, err = node.Self.Sign(t.Hash)
	if err != nil {
		return err
	}
	return nil
}
