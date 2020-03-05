package handler

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"0chain.net/blobbercore/config"
	. "0chain.net/core/logging"
	"0chain.net/core/node"
	"0chain.net/core/transaction"

	"github.com/0chain/gosdk/zcncore"
	"go.uber.org/zap"
)

type WalletCallback struct {
	wg  *sync.WaitGroup
	err string
}

func (wb *WalletCallback) OnWalletCreateComplete(status int, wallet string, err string) {
	wb.err = err
	wb.wg.Done()
}

func RegisterBlobber(ctx context.Context) (string, error) {

	wcb := &WalletCallback{}
	wcb.wg = &sync.WaitGroup{}
	wcb.wg.Add(1)
	err := zcncore.RegisterToMiners(node.Self.GetWallet(), wcb)
	if err != nil {
		return "", err
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

	snBytes, err := json.Marshal(sn)
	if err != nil {
		return "", err
	}
	Logger.Info("Adding blobber to the blockchain.")
	err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS, transaction.ADD_BLOBBER_SC_NAME, string(snBytes), 0)
	if err != nil {
		Logger.Info("Failed during registering blobber to the mining network", zap.String("err:", err.Error()))
		return "", err
	}

	return txn.Hash, nil
}
