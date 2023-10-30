package transaction

import (
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/gosdk/zboxcore/zboxutil"
)

const TXN_SUBMIT_URL = "v1/transaction/put"
const TXN_VERIFY_URL = "v1/transaction/get/confirmation?hash="
const SC_REST_API_URL = "v1/screst/"
const REGISTER_CLIENT = "v1/client/put"

const (
	SLEEP_FOR_TXN_CONFIRMATION = 1
)

var ErrNoTxnDetail = common.NewError("missing_transaction_detail", "No transaction detail was found on any of the sharders")
var MakeSCRestAPICall func(scAddress string, relativePath string, params map[string]string) ([]byte, error) = MakeSCRestAPICallNoHandler

func MakeSCRestAPICallNoHandler(address string, path string, params map[string]string) ([]byte, error) {
	return zboxutil.MakeSCRestAPICall(address, path, params, nil)
}

func VerifyTransaction(txnHash string, chain *chain.Chain) (*Transaction, error) {
	txn, err := NewTransactionEntity()
	if err != nil {
		return nil, err
	}

	txn.Hash = txnHash
	err = txn.Verify()
	if err != nil {
		return nil, err
	}
	return txn, nil
}

// VerifyTransactionWithNonce verifies a transaction with known nonce.
func VerifyTransactionWithNonce(txnHash string, nonce int64) (*Transaction, error) {
	txn, err := NewTransactionEntity()
	if err != nil {
		return nil, err
	}

	txn.Hash = txnHash
	_ = txn.zcntxn.SetTransactionNonce(nonce)

	err = txn.Verify()
	if err != nil {
		return nil, err
	}
	return txn, nil
}
