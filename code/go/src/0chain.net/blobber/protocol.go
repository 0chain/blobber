package blobber

import (
	"encoding/json"
	"time"

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
	VerifyBlobberTransaction(txn_hash string) (string, error)
	VerifyMarker(wm *writemarker.WriteMarker) error
	RedeemMarker()
}

//StorageProtocolImpl - implementation of the storage protocol
type StorageProtocolImpl struct {
	ServerChain  *chain.Chain
	AllocationID string
}

func GetProtocolImpl(allocationID string) StorageProtocol {
	return &StorageProtocolImpl{
		ServerChain:  chain.GetServerChain(),
		AllocationID: allocationID}
}

func (sp *StorageProtocolImpl) RegisterBlobber() (string, error) {
	nodeBytes, _ := json.Marshal(node.Self)
	transaction.SendPostRequestSync(transaction.REGISTER_CLIENT, nodeBytes, sp.ServerChain)
	time.Sleep(3 * time.Second)

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

func (sp *StorageProtocolImpl) VerifyBlobberTransaction(txn_hash string) (string, error) {
	t, err := transaction.VerifyTransaction(txn_hash, sp.ServerChain)
	if err != nil {
		return "", err
	}
	return t.TransactionOutput, nil
}

func (sp *StorageProtocolImpl) VerifyMarker(wm *writemarker.WriteMarker) error {

	if wm == nil {
		return common.NewError("no_write_marker", "No Write Marker was found")
	} else {
		if wm.BlobberID != node.Self.ID {
			return common.NewError("write_marker_validation_failed", "Write Marker is not for the blobber")
		}
		if len(wm.IntentTransactionID) == 0 {
			return common.NewError("write_marker_validation_failed", "Write Marker has no valid intent transaction")
		}
		txnoutput, err := sp.VerifyBlobberTransaction(wm.IntentTransactionID)
		if err != nil {
			return err
		}
		Logger.Info("Transaction out received.", zap.String("txn_output", txnoutput))
		var objmap map[string]*json.RawMessage
		err = json.Unmarshal([]byte(txnoutput), &objmap)
		if err != nil {
			Logger.Error("Error unmarshalling response", zap.Any("error", err))
			return err
		}
		// if wm.DataID != sp.DataID {
		// 	Logger.Error("Validation of DataID failed. ", zap.Any("wm_data_id", wm.DataID), zap.Any("sp_data_id", sp.DataID))
		// 	return common.NewError("write_marker_validation_failed", "Write Marker is not for the data being uploaded")
		// }
		// if wm.AllocationID != sp.AllocationID {
		// 	return common.NewError("write_marker_validation_failed", "Write Marker is not for the same intent transaction")
		// }
	}
	return nil
}

func (sp *StorageProtocolImpl) RedeemMarker() {

}
