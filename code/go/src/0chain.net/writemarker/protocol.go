package writemarker

import (
	"context"
	"encoding/json"
	"time"

	"0chain.net/chain"
	. "0chain.net/logging"
	"0chain.net/transaction"

	"go.uber.org/zap"
)

type CommitConnection struct {
	AllocationRoot     string       `json:"allocation_root"`
	PrevAllocationRoot string       `json:"prev_allocation_root"`
	WriteMarker        *WriteMarker `json:"write_marker"`
}

func (wm *WriteMarkerEntity) RedeemMarker(ctx context.Context) error {

	if len(wm.CloseTxnID) > 0 {
		t, err := transaction.VerifyTransaction(wm.CloseTxnID, chain.GetServerChain())
		if err == nil {
			wm.Status = Committed
			wm.StatusMessage = t.TransactionOutput
			wm.CloseTxnID = t.Hash
			err = wm.Write(ctx)
			return err
		}
	}

	txn := transaction.NewTransactionEntity()
	sn := &CommitConnection{}
	sn.AllocationRoot = wm.WM.AllocationRoot
	sn.PrevAllocationRoot = wm.WM.PreviousAllocationRoot
	sn.WriteMarker = wm.WM

	scData := &transaction.SmartContractTxnData{}
	scData.Name = transaction.CLOSE_CONNECTION_SC_NAME
	scData.InputArgs = sn

	txn.ToClientID = transaction.STORAGE_CONTRACT_ADDRESS
	txn.Value = 0
	txn.TransactionType = transaction.TxnTypeSmartContract
	txnBytes, err := json.Marshal(scData)
	if err != nil {
		Logger.Error("Error encoding sc input", zap.String("err:", err.Error()), zap.Any("scdata", scData))
		wm.Status = Failed
		wm.StatusMessage = "Error encoding sc input. " + err.Error()
		wm.ReedeemRetries++
		wm.Write(ctx)
		return err
	}
	txn.TransactionData = string(txnBytes)

	err = txn.ComputeHashAndSign()
	if err != nil {
		Logger.Error("Signing Failed during sending close connection to the miner. ", zap.String("err:", err.Error()))
		wm.Status = Failed
		wm.StatusMessage = "Signing Failed during sending close connection to the miner. " + err.Error()
		wm.ReedeemRetries++
		wm.Write(ctx)
		return err
	}
	transaction.SendTransactionSync(txn, chain.GetServerChain())
	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
	t, err := transaction.VerifyTransaction(txn.Hash, chain.GetServerChain())
	if err != nil {
		Logger.Error("Error verifying the close connection transaction", zap.String("err:", err.Error()), zap.String("txn", txn.Hash))
		wm.Status = Failed
		wm.StatusMessage = "Error verifying the close connection transaction." + err.Error()
		wm.ReedeemRetries++
		wm.CloseTxnID = txn.Hash
		wm.Write(ctx)
		return err
	}
	wm.Status = Committed
	wm.StatusMessage = t.TransactionOutput
	wm.CloseTxnID = t.Hash
	err = wm.Write(ctx)
	return err
}
