package handler

import (
	"context"
	"encoding/json"
	"time"

	"0chain.net/core/chain"
	. "0chain.net/core/logging"
	"0chain.net/core/node"
	"0chain.net/core/transaction"

	"go.uber.org/zap"
)

func RegisterBlobber(ctx context.Context) (string, error) {
	nodeBytes, _ := json.Marshal(node.Self)
	transaction.SendPostRequestSync(transaction.REGISTER_CLIENT, nodeBytes, chain.GetServerChain())
	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)

	txn := transaction.NewTransactionEntity()

	sn := &transaction.StorageNode{}
	sn.ID = node.Self.GetKey()
	sn.BaseURL = node.Self.GetURLBase()

	scData := &transaction.SmartContractTxnData{}
	scData.Name = transaction.ADD_BLOBBER_SC_NAME
	scData.InputArgs = sn

	txn.ToClientID = transaction.STORAGE_CONTRACT_ADDRESS
	txn.Value = 0
	txn.TransactionType = transaction.TxnTypeSmartContract
	txnBytes, err := json.Marshal(scData)
	if err != nil {
		return "", err
	}
	txn.TransactionData = string(txnBytes)

	err = txn.ComputeHashAndSign()
	if err != nil {
		Logger.Info("Signing Failed during registering blobber to the mining network", zap.String("err:", err.Error()))
		return "", err
	}
	Logger.Info("Adding blobber to the blockchain.", zap.String("txn", txn.Hash))
	transaction.SendTransaction(txn, chain.GetServerChain())
	return txn.Hash, nil
}
