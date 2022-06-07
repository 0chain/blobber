package handler

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
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
	sn.Geolocation = transaction.StorageNodeGeolocation(config.Geolocation())
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
	sn.Terms.MaxOfferDuration = config.Configuration.MaxOfferDuration
	sn.Terms.ChallengeCompletionTime = config.Configuration.ChallengeCompletionTime

	sn.StakePoolSettings.DelegateWallet = config.Configuration.DelegateWallet
	sn.StakePoolSettings.MinStake = config.Configuration.MinStake
	sn.StakePoolSettings.MaxStake = config.Configuration.MaxStake
	sn.StakePoolSettings.NumDelegates = config.Configuration.NumDelegates
	sn.StakePoolSettings.ServiceCharge = config.Configuration.ServiceCharge

	sn.Information.Name = config.Configuration.Name
	sn.Information.Description = config.Configuration.Description
	sn.Information.WebsiteUrl = config.Configuration.WebsiteUrl
	sn.Information.LogoUrl = config.Configuration.LogoUrl
	return sn, nil
}

// RegisterBlobber register blobber if it doesn't registered yet. sync terms and stake pool settings from blockchain if it is registered
func RegisterBlobber(ctx context.Context) error {

	b, err := config.ReloadFromChain(ctx, datastore.GetStore().GetDB())
	if err != nil || b.BaseURL != node.Self.GetURLBase() { // blobber is not registered yet, baseURL is changed
		txnHash, err := sendSmartContractBlobberAdd(ctx)
		if err != nil {
			return err
		}

		t, err := TransactionVerify(txnHash)
		if err != nil {
			logging.Logger.Error("Failed to verify blobber register transaction", zap.Any("err", err), zap.String("txn.Hash", txnHash))
			return err
		}

		logging.Logger.Info("Verified blobber register transaction", zap.String("txn_hash", t.Hash), zap.Any("txn_output", t.TransactionOutput))
		return nil
	}

	return SendHealthCheck()

}

// UpdateBlobber update blobber
func UpdateBlobber(ctx context.Context) error {

	txnHash, err := sendSmartContractBlobberAdd(ctx)
	if err != nil {
		return err
	}

	t, err := TransactionVerify(txnHash)
	if err != nil {
		logging.Logger.Error("Failed to verify blobber update transaction", zap.Any("err", err), zap.String("txn.Hash", txnHash))
		return err
	}

	logging.Logger.Info("Verified blobber update transaction", zap.String("txn_hash", t.Hash), zap.Any("txn_output", t.TransactionOutput))
	return nil

}

func RefreshPriceOnChain(ctx context.Context) error {
	txnHash, err := sendSmartContractBlobberAdd(ctx)
	if err != nil {
		return err
	}

	if t, err := TransactionVerify(txnHash); err != nil {
		logging.Logger.Error("Failed to verify price refresh transaction", zap.Any("err", err), zap.String("txn.Hash", txnHash))
	} else {
		logging.Logger.Info("Verified price refresh transaction", zap.String("txn_hash", t.Hash), zap.Any("txn_output", t.TransactionOutput))
	}

	return err
}

// sendSmartContractBlobberAdd Add or update blobber on blockchain
func sendSmartContractBlobberAdd(ctx context.Context) (string, error) {
	// initialize storage node (ie blobber)
	txn, err := transaction.NewTransactionEntity()
	if err != nil {
		return "", err
	}

	sn, err := getStorageNode()
	if err != nil {
		return "", err
	}

	err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS,
		transaction.ADD_BLOBBER_SC_NAME, sn, 0)
	if err != nil {
		logging.Logger.Error("Failed to set blobber on the blockchain",
			zap.String("err:", err.Error()))
		return "", err
	}

	return txn.Hash, nil
}

// UpdateBlobberOnChain updates latest changes in blobber's settings, capacity,etc.
func UpdateBlobberOnChain(ctx context.Context) error {

	txnHash, err := sendSmartContractBlobberUpdate(ctx)
	if err != nil {
		return err
	}

	if t, err := TransactionVerify(txnHash); err != nil {
		logging.Logger.Error("Failed to verify blobber update transaction", zap.Any("err", err), zap.String("txn.Hash", txnHash))
	} else {
		logging.Logger.Info("Verified blobber update transaction", zap.String("txn_hash", t.Hash), zap.Any("txn_output", t.TransactionOutput))
	}

	return err
}

// sendSmartContractBlobberUpdate update blobber on blockchain
func sendSmartContractBlobberUpdate(ctx context.Context) (string, error) {
	// initialize storage node (ie blobber)
	txn, err := transaction.NewTransactionEntity()
	if err != nil {
		return "", err
	}

	sn, err := getStorageNode()
	if err != nil {
		return "", err
	}

	err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS,
		transaction.UPDATE_BLOBBER_SC_NAME, sn, 0)
	if err != nil {
		logging.Logger.Error("Failed to set blobber on the blockchain",
			zap.String("err:", err.Error()))
		return "", err
	}

	return txn.Hash, nil
}

// ErrBlobberHasRemoved represents service health check error, where the
// blobber has removed (by owner, in case the blobber doesn't provide its
// service anymore). Thus the blobber shouldn't send the health check
// transactions.
var ErrBlobberHasRemoved = errors.New("blobber has been removed")

// ErrBlobberNotFound it is not registered on chain
var ErrBlobberNotFound = errors.New("blobber is not found")

func TransactionVerify(txnHash string) (t *transaction.Transaction, err error) {
	for i := 0; i < util.MAX_RETRIES; i++ {
		time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
		if t, err = transaction.VerifyTransaction(txnHash, chain.GetServerChain()); err == nil {
			return t, nil
		}
	}

	return nil, errors.New("[txn]max retries exceeded with " + txnHash)
}

func WalletRegister() error {
	wcb := &WalletCallback{}
	wcb.wg = &sync.WaitGroup{}
	wcb.wg.Add(1)
	if err := zcncore.RegisterToMiners(node.Self.GetWallet(), wcb); err != nil {
		return err
	}

	return nil
}

// SendHealthCheck send heartbeat to blockchain
func SendHealthCheck() error {
	log.Println("SendHealthCheck")
	txnHash, err := BlobberHealthCheck()
	if err != nil {
		return err
	}
	_, err = TransactionVerify(txnHash)
	if err != nil {
		logging.Logger.Error("Failed to verify blobber health check", zap.Any("err", err), zap.String("txn.Hash", txnHash))
		return err
	}

	return nil
}
