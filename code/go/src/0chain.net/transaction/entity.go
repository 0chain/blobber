package transaction

import (
	"0chain.net/common"
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
const TXN_CONFIRMATION_URL = "v1/transaction/put"
const STORAGE_CONTRACT_ADDRESS = "6dba10422e368813802877a85039d3985d96760ed844092319743fb3a76712d7"
const MAX_TXN_RETRIES = 3
