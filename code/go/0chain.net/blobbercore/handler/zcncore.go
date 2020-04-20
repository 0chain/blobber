package handler

import (
	"sync"

	"github.com/0chain/gosdk/core/common"
	"github.com/0chain/gosdk/zcncore"
)

type ZCNStatus struct {
	wg      *sync.WaitGroup
	success bool
	balance int64
}

func (zcn *ZCNStatus) OnBalanceAvailable(status int, value int64, info string) {
	defer zcn.wg.Done()
	if status == zcncore.StatusSuccess {
		zcn.success = true
	} else {
		zcn.success = false
	}
	zcn.balance = value
}

func (zcn *ZCNStatus) OnTransactionComplete(t *zcncore.Transaction, status int) {
	defer zcn.wg.Done()
	if status == zcncore.StatusSuccess {
		zcn.success = true
	} else {
		zcn.success = false
	}
}

func (zcn *ZCNStatus) OnVerifyComplete(t *zcncore.Transaction, status int) {
	defer zcn.wg.Done()
	if status == zcncore.StatusSuccess {
		zcn.success = true
	} else {
		zcn.success = false
	}
}

func (zcn *ZCNStatus) OnAuthComplete(t *zcncore.Transaction, status int) {}

func CheckBalance() (float64, error) {
	wg := &sync.WaitGroup{}
	statusBar := &ZCNStatus{wg: wg}
	wg.Add(1)
	err := zcncore.GetBalance(statusBar)
	if err != nil {
		return 0, common.NewError("check_balance_failed", "Call to GetBalance failed with err: "+err.Error())
	}
	wg.Wait()
	if statusBar.success == false {
		return 0, nil
	}
	return zcncore.ConvertToToken(statusBar.balance), nil
}

func CallFaucet() error {
	wg := &sync.WaitGroup{}
	statusBar := &ZCNStatus{wg: wg}
	txn, err := zcncore.NewTransaction(statusBar, 0)
	if err != nil {
		return common.NewError("call_faucet_failed", "Failed to create new transaction with err: "+err.Error())
	}
	wg.Add(1)
	err = txn.ExecuteSmartContract(zcncore.FaucetSmartContractAddress, "pour", "Blobber Registration", zcncore.ConvertToValue(0))
	if err != nil {
		return common.NewError("call_faucet_failed", "Failed to execute smart contract with err: "+err.Error())
	}
	wg.Wait()
	if statusBar.success == false {
		return common.NewError("call_faucet_failed", "Failed to execute smart contract with statusBar success failed")
	}
	statusBar.success = false
	wg.Add(1)
	err = txn.Verify()
	if err != nil {
		return common.NewError("call_faucet_failed", "Failed to verify smart contract with err: "+err.Error())
	}
	wg.Wait()
	if statusBar.success == false {
		return common.NewError("call_faucet_failed", "Failed to verify smart contract with statusBar success failed")
	}
	return nil
}
