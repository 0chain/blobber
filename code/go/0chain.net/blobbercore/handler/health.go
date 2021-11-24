package handler

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"go.uber.org/zap"
)

var blobberHealthCheckErr error

func SetBlobberHealthError(err error) {
	blobberHealthCheckErr = err
}

func GetBlobberHealthError() error{
	return blobberHealthCheckErr
}

func HealthCheckOnChain() error {
	txnHash, err := blobberHealthCheck(common.GetRootContext())
	if err != nil {
		if err == ErrBlobberHasRemoved {
			return nil
		} else {
			return err
		}
	}

	if t, err := TransactionVerify(txnHash); err != nil {
		logging.Logger.Error("Failed to verify blobber health check", zap.Any("err", err), zap.String("txn.Hash", txnHash))
	} else {
		logging.Logger.Info("Verified blobber health check", zap.String("txn_hash", t.Hash), zap.Any("txn_output", t.TransactionOutput))
	}

	return err
}

func blobberHealthCheck(ctx context.Context) (string, error) {
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