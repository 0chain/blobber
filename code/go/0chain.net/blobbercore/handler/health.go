package handler

import (
	"sync"

	coreTxn "github.com/0chain/gosdk/core/transaction"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"go.uber.org/zap"
)

var (
	// blobberHealthCheckError use it on stats page
	blobberHealthCheckError error
	blobberHealthCheckMutex sync.RWMutex
)

func setBlobberHealthCheckError(err error) {
	blobberHealthCheckMutex.Lock()
	blobberHealthCheckError = err
	blobberHealthCheckMutex.Unlock()
}

func getBlobberHealthCheckError() error {
	blobberHealthCheckMutex.RLock()
	err := blobberHealthCheckError
	blobberHealthCheckMutex.RUnlock()
	return err
}

func BlobberHealthCheck() (*coreTxn.Transaction, error) {
	if config.Configuration.Capacity == 0 {

		setBlobberHealthCheckError(ErrBlobberHasRemoved)
		return nil, ErrBlobberHasRemoved
	}

	_, _, _, txn, err := coreTxn.SmartContractTxn(transaction.STORAGE_CONTRACT_ADDRESS, coreTxn.SmartContractTxnData{
		Name:      transaction.BLOBBER_HEALTH_CHECK,
		InputArgs: common.Now(),
	}, true)
	if err != nil {
		logging.Logger.Error("Failed to health check blobber on the blockchain",
			zap.Error(err))
		setBlobberHealthCheckError(err)

		return nil, err
	}

	setBlobberHealthCheckError(nil)

	return txn, nil
}

func ValidatorHealthCheck() (*coreTxn.Transaction, error) {
	_, _, _, txn, err := coreTxn.SmartContractTxn(transaction.STORAGE_CONTRACT_ADDRESS, coreTxn.SmartContractTxnData{
		Name:      transaction.VALIDATOR_HEALTH_CHECK,
		InputArgs: common.Now(),
	}, true)
	return txn, err
}
