package handler

import (
	"encoding/json"
	"sync"

	"github.com/0chain/gosdk/core/common"
	"github.com/0chain/gosdk/zcncore"
)

type ZCNStatus struct {
	wg      *sync.WaitGroup
	success bool
	balance int64
	info    string
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

func (zcn *ZCNStatus) OnInfoAvailable(op int, status int, info string, err string) {
	defer zcn.wg.Done()
	if status == zcncore.StatusSuccess {
		zcn.success = true
	} else {
		zcn.success = false
	}
	zcn.info = info
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
	if !statusBar.success {
		return 0, nil
	}
	return zcncore.ConvertToToken(statusBar.balance), nil
}

func GetBlobbers() ([]*zcncore.Blobber, error) {
	var info struct {
		Nodes []*zcncore.Blobber
	}

	wg := &sync.WaitGroup{}
	statusBar := &ZCNStatus{wg: wg}
	wg.Add(1)

	err := zcncore.GetBlobbers(statusBar)
	if err != nil {
		return info.Nodes, common.NewError("get_blobbers_failed", "Call to GetBlobbers failed with err: "+err.Error())
	}
	wg.Wait()

	if !statusBar.success {
		return info.Nodes, nil
	}

	if err = json.Unmarshal([]byte(statusBar.info), &info); err != nil {
		return info.Nodes, common.NewError("get_blobbers_failed", "Decoding response to GetBlobbers failed with err: "+err.Error())
	}

	return info.Nodes, nil
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
	if !statusBar.success {
		return common.NewError("call_faucet_failed", "Failed to execute smart contract with statusBar success failed")
	}
	statusBar.success = false
	wg.Add(1)
	err = txn.Verify()
	if err != nil {
		return common.NewError("call_faucet_failed", "Failed to verify smart contract with err: "+err.Error())
	}
	wg.Wait()
	if !statusBar.success {
		return common.NewError("call_faucet_failed", "Failed to verify smart contract with statusBar success failed")
	}
	return nil
}

func Transfer(token float64, clientID string) error {
	wg := &sync.WaitGroup{}
	statusBar := &ZCNStatus{wg: wg}
	txn, err := zcncore.NewTransaction(statusBar, 0)
	if err != nil {
		return common.NewError("call_transfer_failed", "Failed to create new transaction with err: "+err.Error())
	}
	wg.Add(1)
	err = txn.Send(clientID, zcncore.ConvertToValue(token), "Blobber delegate transfer")
	if err != nil {
		return common.NewError("call_transfer_failed", "Failed to send tokens with err: "+err.Error())
	}
	wg.Wait()
	if !statusBar.success {
		return common.NewError("call_transfer_failed", "Failed to send tokens with statusBar success failed")
	}
	statusBar.success = false
	wg.Add(1)
	err = txn.Verify()
	if err != nil {
		return common.NewError("call_transfer_failed", "Failed to verify send transaction with err: "+err.Error())
	}
	wg.Wait()
	if !statusBar.success {
		return common.NewError("call_transfer_failed", "Failed to verify send transaction with statusBar success failed")
	}
	return nil

}
