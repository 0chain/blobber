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

func BlobberHealthCheck() (string, error) {
	if config.Configuration.Capacity == 0 {

		setBlobberHealthCheckError(ErrBlobberHasRemoved)
		return "", ErrBlobberHasRemoved
	}

	txn, err := transaction.NewTransactionEntity()
	if err != nil {
		setBlobberHealthCheckError(err)
		return "", err
	}

	err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS,
		transaction.BLOBBER_HEALTH_CHECK, common.Now(), 0)
	if err != nil {
		logging.Logger.Info("Failed to health check on the blockchain",
			zap.String("err:", err.Error()))
		setBlobberHealthCheckError(err)

		return "", err
	}

	setBlobberHealthCheckError(err)
	return txn.Hash, nil
}
