package handler

import (
	"sync"

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

func BlobberHealthCheck() (*transaction.Transaction, error) {
	if config.Configuration.Capacity == 0 {

		setBlobberHealthCheckError(ErrBlobberHasRemoved)
		return nil, ErrBlobberHasRemoved
	}

	txn, err := transaction.NewTransactionEntity()
	if err != nil {
		setBlobberHealthCheckError(err)
		return nil, err
	}

	err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS,
		transaction.BLOBBER_HEALTH_CHECK, common.Now(), 0)
	if err != nil {
		logging.Logger.Error("Failed to health check blobber on the blockchain",
			zap.Error(err))
		setBlobberHealthCheckError(err)

		return nil, err
	}

	setBlobberHealthCheckError(nil)

	return txn, nil
}

func ValidatorHealthCheck() (*transaction.Transaction, error) {

	txn, err := transaction.NewTransactionEntity()

	if err != nil {

		return nil, err
	}

	if err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS, transaction.VALIDATOR_HEALTH_CHECK, common.Now(), 0); err != nil {
		logging.Logger.Error("Failed to health check validator on the blockchain",
			zap.Error(err))
		return nil, err
	}

	return txn, err
}
