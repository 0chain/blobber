package handler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"github.com/0chain/blobber/code/go/0chain.net/core/util"
	"github.com/0chain/gosdk/zcncore"
	"go.uber.org/zap"
)

const (
	KB = 1024      // kilobyte
	MB = 1024 * KB // megabyte
	GB = 1024 * MB // gigabyte
)

type WalletCallback struct {
	wg  *sync.WaitGroup
	err string
}

func (wb *WalletCallback) OnWalletCreateComplete(status int, wallet, err string) {
	wb.err = err
	wb.wg.Done()
}

func getStorageNode() (*transaction.StorageNode, error) {
	var err error
	sn := &transaction.StorageNode{}
	sn.ID = node.Self.ID
	sn.BaseURL = node.Self.GetURLBase()
	if err != nil {
		return nil, err
	}
	if config.Configuration.AutomaticUpdate {
		sn.Capacity = int64(filestore.GetFileStore().GetCurrentDiskCapacity())
	} else {
		sn.Capacity = config.Configuration.Capacity
	}

	readPrice := config.Configuration.ReadPrice
	writePrice := config.Configuration.WritePrice
	if config.Configuration.PriceInUSD {
		readPrice, err = zcncore.ConvertUSDToToken(readPrice)
		if err != nil {
			return nil, err
		}

		writePrice, err = zcncore.ConvertUSDToToken(writePrice)
		if err != nil {
			return nil, err
		}
	}
	sn.Terms.ReadPrice = zcncore.ConvertToValue(readPrice)
	sn.Terms.WritePrice = zcncore.ConvertToValue(writePrice)
	sn.Terms.MinLockDemand = config.Configuration.MinLockDemand

	sn.StakePoolSettings.DelegateWallet = config.Configuration.DelegateWallet
	sn.StakePoolSettings.NumDelegates = config.Configuration.NumDelegates
	sn.StakePoolSettings.ServiceCharge = config.Configuration.ServiceCharge

	return sn, nil
}

// RegisterBlobber register blobber if it is not registered yet
func RegisterBlobber(ctx context.Context) error {
	err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		_, e := config.ReloadFromChain(ctx, datastore.GetStore().GetDB())
		return e
	})

	if err != nil { // blobber is not registered yet
		txn, err := sendSmartContractBlobberAdd(ctx)
		if err != nil {
			logging.Logger.Error("Error when sending add request to blockchain", zap.Any("err", err))
			return err
		}

		t, err := TransactionVerify(txn)
		if err != nil {
			logging.Logger.Error("Failed to verify blobber register transaction", zap.Any("err", err), zap.String("txn.Hash", txn.Hash))
			return err
		}

		logging.Logger.Info("Verified blobber register transaction", zap.String("txn_hash", t.Hash), zap.Any("txn_output", t.TransactionOutput))
		return nil
	}

	txnHash, err := SendHealthCheck(common.ProviderTypeBlobber)
	if err != nil {
		logging.Logger.Error("Failed to send healthcheck transaction", zap.String("txn_hash", txnHash))
		return err
	}

	return nil
}

func RefreshPriceOnChain(ctx context.Context) error {
	txn, err := sendSmartContractBlobberAdd(ctx)
	if err != nil {
		return err
	}

	if t, err := TransactionVerify(txn); err != nil {
		logging.Logger.Error("Failed to verify price refresh transaction", zap.Any("err", err), zap.String("txn.Hash", txn.Hash))
	} else {
		logging.Logger.Info("Verified price refresh transaction", zap.String("txn_hash", t.Hash), zap.Any("txn_output", t.TransactionOutput))
	}

	return err
}

// sendSmartContractBlobberAdd Add or update blobber on blockchain
func sendSmartContractBlobberAdd(ctx context.Context) (*transaction.Transaction, error) {
	// initialize storage node (ie blobber)
	txn, err := transaction.NewTransactionEntity()
	if err != nil {
		return nil, err
	}

	sn, err := getStorageNode()
	if err != nil {
		return nil, err
	}

	err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS,
		transaction.ADD_BLOBBER_SC_NAME, sn, 0)
	if err != nil {
		logging.Logger.Error("Failed to set blobber on the blockchain",
			zap.String("err:", err.Error()))
		return nil, err
	}

	return txn, nil
}

// ErrBlobberHasRemoved represents service health check error, where the
// blobber has removed (by owner, in case the blobber doesn't provide its
// service anymore). Thus the blobber shouldn't send the health check
// transactions.
var ErrBlobberHasRemoved = errors.New("blobber has been removed")

// ErrBlobberNotFound it is not registered on chain
var ErrBlobberNotFound = errors.New("blobber is not found")

// ErrValidatorHasRemoved represents service health check error, where the
// Validator has removed (by owner, in case the Validator doesn't provide its
// service anymore). Thus the Validator shouldn't send the health check
// transactions.
var ErrValidatorHasRemoved = errors.New("validator has been removed")

// ErrValidatorNotFound it is not registered on chain
var ErrValidatorNotFound = errors.New("validator is not found")

func TransactionVerify(txn *transaction.Transaction) (t *transaction.Transaction, err error) {
	msg := fmt.Sprintf("Verifying transaction: max_retries: %d", util.MAX_RETRIES)
	logging.Logger.Info(msg)
	for i := 0; i < util.MAX_RETRIES; i++ {
		time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
		if t, err = transaction.VerifyTransactionWithNonce(txn.Hash, txn.GetTransaction().GetTransactionNonce()); err == nil {
			return t, nil
		}
	}

	return nil, errors.New("[txn]max retries exceeded with " + txn.Hash)
}

// SendHealthCheck send heartbeat to blockchain
func SendHealthCheck(provider common.ProviderType) (string, error) {

	var txn *transaction.Transaction
	var err error

	switch provider {
	case common.ProviderTypeBlobber:
		txn, err = BlobberHealthCheck()
	case common.ProviderTypeValidator:
		txn, err = ValidatorHealthCheck()
	default:
		return "", errors.New("unknown provider type")
	}

	if err != nil {
		return "", err
	}

	_, err = TransactionVerify(txn)

	return txn.Hash, err
}
