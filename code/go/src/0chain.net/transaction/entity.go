package transaction

import (
	"0chain.net/common"
)

//Transaction entity that encapsulates the transaction related data and meta data
type Transaction struct {
	Hash      string
	Version   string
	ClientID  string
	PublicKey string

	ToClientID      string
	ChainID         string
	TransactionData string
	Value           int64
	Signature       string
	CreationDate    common.Timestamp

	TransactionType int

	// a parent transaction introdcues certain state and this state is managed as new transactions are created referencing to this parent transaction
	ParentTransactionHash string
}
