package handler

import (
	"sync"
	"time"
	"errors"
	"context"
	"encoding/json"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"github.com/0chain/blobber/code/go/0chain.net/core/util"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"

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

func (wb *WalletCallback) OnWalletCreateComplete(status int, wallet string, err string) {
	wb.err = err
	wb.wg.Done()
}

// size in gigabytes
func sizeInGB(size int64) float64 { //nolint:unused,deadcode // might be used later?
	return float64(size) / GB
}

type apiResp struct { //nolint:unused,deadcode // might be used later?
	ok   bool
	resp string
}

func (ar *apiResp) decode(val interface{}) (err error) { //nolint:unused,deadcode // might be used later?
	if err = ar.err(); err != nil {
		return
	}
	return json.Unmarshal([]byte(ar.resp), val)
}

func (ar *apiResp) err() error { //nolint:unused,deadcode // might be used later?
	if !ar.ok {
		return errors.New(ar.resp)
	}
	return nil
}

func getStorageNode() (*transaction.StorageNode, error) {
	var err error
	sn := &transaction.StorageNode{}
	sn.ID = node.Self.ID
	sn.BaseURL = node.Self.GetURLBase()
	sn.Geolocation = transaction.StorageNodeGeolocation(config.Geolocation())
	sn.Capacity = config.Configuration.Capacity
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
	return sn, nil
}

// Add or update blobber on blockchain
func BlobberAdd(ctx context.Context) (string, error) {
	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)

	// initialize storage node (ie blobber)
	txn, err := transaction.NewTransactionEntity()
	if err != nil {
		return "", err
	}

	sn, err := getStorageNode()
	if err != nil {
		return "", err
	}

	snBytes, err := json.Marshal(sn)
	if err != nil {
		return "", err
	}

	Logger.Info("Adding or updating on the blockchain")

	err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS,
		transaction.ADD_BLOBBER_SC_NAME, string(snBytes), 0)
	if err != nil {
		Logger.Info("Failed to set blobber on the blockchain",
			zap.String("err:", err.Error()))
		return "", err
	}

	return txn.Hash, nil
}

// ErrBlobberHasRemoved represents service health check error, where the
// blobber has removed (by owner, in case the blobber doesn't provide its
// service anymore). Thus the blobber shouldn't send the health check
// transactions.
var ErrBlobberHasRemoved = errors.New("blobber has removed")

func BlobberHealthCheck(ctx context.Context) (string, error) {
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
		Logger.Info("Failed to health check on the blockchain",
			zap.String("err:", err.Error()))
		return "", err
	}

	return txn.Hash, nil
}

func TransactionVerify(txnHash string) (t *transaction.Transaction, err error) {
	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)

	for i := 0; i < util.MAX_RETRIES; i++ {
		time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
		if t, err = transaction.VerifyTransaction(txnHash, chain.GetServerChain()); err == nil {
			return t, nil
		}
	}

	return
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
