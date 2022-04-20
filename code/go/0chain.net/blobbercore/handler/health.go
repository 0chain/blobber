package handler

import (
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"go.uber.org/zap"
)

var blobberHealthCheckErr error

func BlobberHealthCheck() (string, error) {
	if config.Configuration.Capacity == 0 {
		blobberHealthCheckErr = ErrBlobberHasRemoved
		return "", ErrBlobberHasRemoved
	}

	txn, err := transaction.NewTransactionEntity()
	if err != nil {
		blobberHealthCheckErr = err
		return "", err
	}

	err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS,
		transaction.BLOBBER_HEALTH_CHECK, "", 0)
	if err != nil {
		logging.Logger.Info("Failed to health check on the blockchain",
			zap.String("err:", err.Error()))
		blobberHealthCheckErr = err

		return "", err
	}

	blobberHealthCheckErr = nil
	return txn.Hash, nil
}
