package readmarker

import (
	"context"
	"encoding/json"
	"time"

	"0chain.net/chain"
	. "0chain.net/logging"
	"0chain.net/transaction"

	"go.uber.org/zap"
)

type ReadRedeem struct {
	ReadMarker *ReadMarker `json:"read_marker"`
}

func (rmEntity *ReadMarkerEntity) RedeemReadMarker(ctx context.Context, rmStatus *ReadMarkerStatus) error {
	txn := transaction.NewTransactionEntity()
	rm := rmEntity.LatestRM

	sn := &ReadRedeem{}
	sn.ReadMarker = rm
	scData := &transaction.SmartContractTxnData{}

	scData.Name = transaction.READ_REDEEM
	scData.InputArgs = sn

	txn.ToClientID = transaction.STORAGE_CONTRACT_ADDRESS
	txn.Value = 0
	txn.TransactionType = transaction.TxnTypeSmartContract
	txnBytes, err := json.Marshal(scData)
	if err != nil {
		Logger.Error("Error encoding sc input", zap.String("err:", err.Error()), zap.Any("scdata", scData))
		return err
	}
	txn.TransactionData = string(txnBytes)

	err = txn.ComputeHashAndSign()
	if err != nil {
		Logger.Error("Signing Failed during read redeem. ", zap.String("err:", err.Error()))
		return err
	}
	transaction.SendTransactionSync(txn, chain.GetServerChain())
	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
	t, err := transaction.VerifyTransaction(txn.Hash, chain.GetServerChain())
	if err != nil {
		Logger.Error("Error verifying the read redeem transaction", zap.String("err:", err.Error()), zap.String("txn", txn.Hash))
		return err
	}

	rmStatus.LastRedeemTxnID = t.Hash
	rmStatus.LastestRedeemedRM = rm
	rmStatus.StatusMessage = t.TransactionOutput
	err = rmStatus.Write(ctx)
	return err
}
