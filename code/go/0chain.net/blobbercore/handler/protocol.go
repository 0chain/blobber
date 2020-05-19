package handler

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"0chain.net/blobbercore/config"
	. "0chain.net/core/logging"
	"0chain.net/core/node"
	"0chain.net/core/transaction"

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
func sizeInGB(size int64) float64 {
	return float64(size) / GB
}

// stakePoolInfoCallback waits for stake pool info response
type stakePoolInfoCallback func(op int, status int, info string, err string)

func (spic stakePoolInfoCallback) OnInfoAvailable(op int, status int,
	info string, err string) {

	spic(op, status, info, err)
}

type apiResp struct {
	ok   bool
	resp string
}

func (ar *apiResp) decode(val interface{}) (err error) {
	if err = ar.err(); err != nil {
		return
	}
	return json.Unmarshal([]byte(ar.resp), val)
}

func (ar *apiResp) err() error {
	if !ar.ok {
		return errors.New(ar.resp)
	}
	return nil
}

// short stake pool statistic, excluding offers details
type stakePoolInfo struct {
	Locked      int64 `json:"locked"`
	Unlocked    int64 `json:"unlocked"`
	OffersTotal int64 `json:"offers_total"`
}

// calculate required number of tokens to lock
func (spi *stakePoolInfo) requiredStake(stake int64) int64 {
	if spi == nil {
		return stake // can't calculate
	}

	if spi.Unlocked+spi.Locked >= stake {
		return 0 // no more tokens needed
	}

	// TODO (sfxdx): punishments

	return stake - (spi.Unlocked + spi.Locked) // excluding tokens already have
}

// request stake pool information from blockchain
func getStakePoolInfo(id string) (spi *stakePoolInfo, err error) {
	var resp = make(chan apiResp, 1)
	zcncore.GetStakePoolStat(
		stakePoolInfoCallback(func(_ int, status int, info string, err string) {
			var ar apiResp
			ar.ok = (status == zcncore.StatusSuccess)

			if ar.ok {
				ar.resp = info
			} else {
				ar.resp = err
			}

			resp <- ar
		}), id)

	var ar = <-resp
	spi = new(stakePoolInfo)
	if err = ar.decode(spi); err != nil {
		return nil, err
	}
	return
}

func RegisterBlobber(ctx context.Context) (string, error) {

	wcb := &WalletCallback{}
	wcb.wg = &sync.WaitGroup{}
	wcb.wg.Add(1)
	err := zcncore.RegisterToMiners(node.Self.GetWallet(), wcb)
	if err != nil {
		return "", err
	}

	if config.Development() {
		CheckForFunds()
	}

	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)

	txn, err := transaction.NewTransactionEntity()
	if err != nil {
		return "", err
	}

	sn := &transaction.StorageNode{}
	sn.ID = node.Self.GetKey()
	sn.BaseURL = node.Self.GetURLBase()
	sn.Capacity = config.Configuration.Capacity
	sn.Terms.ReadPrice = zcncore.ConvertToValue(config.Configuration.ReadPrice)
	sn.Terms.WritePrice = zcncore.ConvertToValue(config.Configuration.WritePrice)
	sn.Terms.MinLockDemand = config.Configuration.MinLockDemand
	sn.Terms.MaxOfferDuration = config.Configuration.MaxOfferDuration
	sn.Terms.ChallengeCompletionTime = config.Configuration.ChallengeCompletionTime

	sn.DelegateWallet = config.Configuration.DelegateWallet

	snBytes, err := json.Marshal(sn)
	if err != nil {
		return "", err
	}
	Logger.Info("Adding blobber to the blockchain.")
	err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS,
		transaction.ADD_BLOBBER_SC_NAME, string(snBytes), 0)
	if err != nil {
		Logger.Info("Failed during registering blobber to the mining network",
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
	Logger.Info("Blobber health check to the blockchain.")
	err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS,
		transaction.BLOBBER_HEALTH_CHECK, "", 0)
	if err != nil {
		Logger.Info("Failed during blobber health check to the mining network",
			zap.String("err:", err.Error()))
		return "", err
	}

	return txn.Hash, nil
}

func CheckForFunds() {
	balance, err := CheckBalance()
	if err != nil {
		Logger.Error("Failed to check for funds", zap.Error(err))
		panic("Unable to get balance")
	}
	for balance < config.Configuration.FaucetMinimumBalance {
		Logger.Info("Doesn't have minimum balance required, Calling faucet")
		err = CallFaucet()
		if err != nil {
			Logger.Error("Failed to call faucet", zap.Error(err))
			continue
		}
		balance, err = CheckBalance()
		if err != nil {
			Logger.Error("Failed to check for funds", zap.Error(err))
			panic("Unable to get balance")
		}
		Logger.Info("Faucet successfully called", zap.Any("current_balance", balance))
	}

	err = Transfer(balance, config.Configuration.DelegateWallet)
	if err != nil {
		Logger.Error("Failed to transfer funds to delegate wallet", zap.Error(err))
		panic("Unable to transfer funds to delegate wallet")
	}
	Logger.Info("Transfer successfully called", zap.Any("tokens_transfered", balance))
}
