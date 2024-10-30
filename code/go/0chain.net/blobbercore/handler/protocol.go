package handler

import (
	"context"
	"errors"
	"github.com/0chain/gosdk/core/client"
	coreTxn "github.com/0chain/gosdk/core/transaction"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
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

	sn.StakePoolSettings.DelegateWallet = config.Configuration.DelegateWallet
	sn.StakePoolSettings.NumDelegates = config.Configuration.NumDelegates
	sn.StakePoolSettings.ServiceCharge = config.Configuration.ServiceCharge

	sn.IsEnterprise = config.Configuration.IsEnterprise
	sn.StorageVersion = allocation.StorageV2

	return sn, nil
}

// RegisterBlobber register blobber if it is not registered yet
func RegisterBlobber(ctx context.Context) error {
	err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		_, e := config.ReloadFromChain(ctx, datastore.GetStore().GetDB())
		return e
	})

	if err != nil { // blobber is not registered yet
		txn, err := sendSmartContractBlobberAdd()
		if err != nil {
			logging.Logger.Error("Error in add blobber", zap.Any("err", err))
			return err
		}

		logging.Logger.Info("Verified blobber register transaction", zap.String("txn_hash", txn.Hash), zap.Any("txn_output", txn.TransactionOutput))
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
	txn, err := sendSmartContractBlobberAdd()
	if err != nil {
		logging.Logger.Error("Failed to verify price refresh transaction", zap.Any("err", err), zap.String("txn.Hash", txn.Hash))
		return err
	}

	logging.Logger.Info("Verified price refresh transaction", zap.String("txn_hash", txn.Hash), zap.Any("txn_output", txn.TransactionOutput))
	return nil
}

// sendSmartContractBlobberAdd Add or update blobber on blockchain
func sendSmartContractBlobberAdd() (*coreTxn.Transaction, error) {

	sn, err := getStorageNode()
	if err != nil {
		return nil, err
	}

	_, _, _, txn, err := coreTxn.SmartContractTxn(transaction.STORAGE_CONTRACT_ADDRESS, coreTxn.SmartContractTxnData{
		Name:      transaction.ADD_BLOBBER_SC_NAME,
		InputArgs: sn,
	}, true)
	if err != nil {
		logging.Logger.Error("Failed to set blobber on the blockchain",
			zap.String("err:", err.Error()), zap.Any("Txn", txn), zap.Any("ClientFee", client.TxnFee()))
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

// SendHealthCheck send heartbeat to blockchain
func SendHealthCheck(provider common.ProviderType) (string, error) {

	var hash string
	var err error

	switch provider {
	case common.ProviderTypeBlobber:
		hash, err = BlobberHealthCheck()
	case common.ProviderTypeValidator:
		hash, err = ValidatorHealthCheck()
	default:
		return "", errors.New("unknown provider type")
	}

	return hash, err
}
