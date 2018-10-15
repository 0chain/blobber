package blobber

import (
	"encoding/json"

	"0chain.net/chain"
	"0chain.net/common"
	. "0chain.net/logging"
	"0chain.net/node"
	"0chain.net/transaction"
	"0chain.net/writemarker"
	"go.uber.org/zap"
)

//StorageProtocol - interface for the storage protocol
type StorageProtocol interface {
	RegisterBlobber() (string, error)
	VerifyAllocationTransaction()
	VerifyBlobberTransaction()
	VerifyMarker() error
	RedeemMarker()
}

//StorageProtocolImpl - implementation of the storage protocol
type StorageProtocolImpl struct {
	ServerChain  *chain.Chain
	AllocationID string
	IntentTxnID  string
	DataID       string
	WriteMarker  *writemarker.WriteMarker
}

func GetProtocolImpl(allocationID string, intentTxn string, dataID string, wm *writemarker.WriteMarker) StorageProtocol {
	return &StorageProtocolImpl{
		ServerChain:  chain.GetServerChain(),
		AllocationID: allocationID,
		IntentTxnID:  intentTxn,
		DataID:       dataID,
		WriteMarker:  wm}
}

func (sp *StorageProtocolImpl) RegisterBlobber() (string, error) {
	nodeBytes, _ := json.Marshal(node.Self)
	transaction.SendPostRequest(transaction.REGISTER_CLIENT, nodeBytes, sp.ServerChain)
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
	transaction.SendTransaction(txn, sp.ServerChain)
	return txn.Hash, nil
}

func (sp *StorageProtocolImpl) VerifyAllocationTransaction() {

}

func (sp *StorageProtocolImpl) VerifyBlobberTransaction() {

}

func (sp *StorageProtocolImpl) VerifyMarker() error {
	wm := sp.WriteMarker
	if wm == nil {
		return common.NewError("no_write_marker", "No Write Marker was found")
	} else {
		if wm.BlobberID != node.Self.ID {
			return common.NewError("write_marker_validation_failed", "Write Marker is not for the blobber")
		}
		if wm.DataID != sp.DataID {
			return common.NewError("write_marker_validation_failed", "Write Marker is not for the data being uploaded")
		}
		if wm.IntentTransactionID != sp.IntentTxnID {
			return common.NewError("write_marker_validation_failed", "Write Marker is not for the same intent transaction")
		}
	}
	return nil
}

func (sp *StorageProtocolImpl) RedeemMarker() {

}
