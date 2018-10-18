package transaction

import (
	"fmt"

	"0chain.net/chain"
	"0chain.net/common"
	"0chain.net/encryption"
	"0chain.net/node"
	"0chain.net/writemarker"
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

type CloseConnection struct {
	DataID      string                  `json:"data_id"`
	MerkleRoot  string                  `json:"merkle_root"`
	WriteMarker writemarker.WriteMarker `json:"write_marker"`
}

type StorageConnection struct {
	ClientPublicKey string                     `json:"client_public_key"`
	AllocationID    string                     `json:"allocation_id"`
	Status          int                        `json:"status"`
	BlobberData     []StorageConnectionBlobber `json:"blobber_data"`
}

type StorageConnectionBlobber struct {
	BlobberID         string `json:"blobber_id"`
	DataID            string `json:"data_id"`
	Size              int64  `json:"size"`
	MerkleRoot        string `json:"merkle_root"`
	OpenConnectionTxn string `json:"open_connection_txn"`
	AllocationID      string `json:"allocation_id"`
}

const ADD_BLOBBER_SC_NAME = "add_blobber"
const CLOSE_CONNECTION_SC_NAME = "close_connection"

const STORAGE_CONTRACT_ADDRESS = "6dba10422e368813802877a85039d3985d96760ed844092319743fb3a76712d7"

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
