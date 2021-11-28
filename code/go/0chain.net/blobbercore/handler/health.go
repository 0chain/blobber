package handler

import (
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"go.uber.org/zap"
)

var blobberHealthCheckErr error

func SetBlobberHealthError(err error) {
	blobberHealthCheckErr = err
}

func GetBlobberHealthError() error {
	return blobberHealthCheckErr
}

func BlobberHealthCheck() (string, error) {
	if config.Configuration.Capacity == 0 {
		return "", ErrBlobberHasRemoved
	}

	txn, err := transaction.NewTransactionEntity()
	if err != nil {
		return "", err
	}

	err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS,
		transaction.BLOBBER_HEALTH_CHECK, "", 0)
	if err != nil {
		logging.Logger.Info("Failed to health check on the blockchain",
			zap.String("err:", err.Error()))
		return "", err
	}

	return txn.Hash, nil
}
