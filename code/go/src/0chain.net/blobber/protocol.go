package blobber

import (
	"encoding/json"
	"fmt"
	"time"

	"0chain.net/badgerdbstore"
	"0chain.net/encryption"

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
	VerifyBlobberTransaction(txn_hash string) (*transaction.StorageConnection, error)
	VerifyMarker(wm *writemarker.WriteMarker, sc *transaction.StorageConnection) error
	RedeemMarker(wm *writemarker.WriteMarkerEntity)
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
	transaction.SendTransaction(txn, sp.ServerChain)
	return txn.Hash, nil
}

func (sp *StorageProtocolImpl) VerifyAllocationTransaction() {

}

func (sp *StorageProtocolImpl) VerifyBlobberTransaction(txn_hash string) (*transaction.StorageConnection, error) {
	if len(txn_hash) == 0 {
		return nil, common.NewError("open_connection_txn_invalid", "Open connection Txn is blank. ")
	}

	t, err := transaction.VerifyTransaction(txn_hash, sp.ServerChain)
	if err != nil {
		return nil, common.NewError("open_connection_txn_invalid", "Open connection Txn could not be found. "+err.Error())
	}
	var storageConnection transaction.StorageConnection
	err = json.Unmarshal([]byte(t.TransactionOutput), &storageConnection)
	if err != nil {
		return nil, common.NewError("transaction_output_decode_error", "Error decoding the transaction output."+err.Error())
	}
	foundBlobber := false
	for _, blobberConnection := range storageConnection.BlobberData {
		if blobberConnection.BlobberID == node.Self.ID {
			foundBlobber = true
			break
		}
	}
	if !foundBlobber {
		return nil, common.NewError("invalid_blobber", "Blobber is not part of the open connection transaction")
	}
	return &storageConnection, nil
}

func (sp *StorageProtocolImpl) VerifyMarker(wm *writemarker.WriteMarker, storageConnection *transaction.StorageConnection) error {

	if wm == nil {
		return common.NewError("no_write_marker", "No Write Marker was found")
	} else {
		if wm.BlobberID != node.Self.ID {
			return common.NewError("write_marker_validation_failed", "Write Marker is not for the blobber")
		}
		if len(wm.IntentTransactionID) == 0 {
			return common.NewError("write_marker_validation_failed", "Write Marker has no valid intent transaction")
		}
		txnoutput := storageConnection
		var err error
		if txnoutput == nil {
			txnoutput, err = sp.VerifyBlobberTransaction(wm.IntentTransactionID)
			if err != nil {
				return err
			}
		}

		Logger.Info("Transaction out received.", zap.Any("storage_connection", txnoutput))

		foundDataID := false
		var wmBlobberConnection *transaction.StorageConnectionBlobber = nil

		for _, blobberConnection := range txnoutput.BlobberData {
			if blobberConnection.BlobberID == node.Self.ID {
				if blobberConnection.DataID == wm.DataID {
					foundDataID = true
					wmBlobberConnection = &blobberConnection
					break
				}
			}
		}
		if !foundDataID {
			return common.NewError("write_marker_validation_failed", "Write Marker is not for the data being uploaded")
		}
		if txnoutput.AllocationID != sp.AllocationID {
			return common.NewError("write_marker_validation_failed", "Write Marker is not for the same allocation transaction")
		}
		if wmBlobberConnection != nil && wmBlobberConnection.OpenConnectionTxn != wm.IntentTransactionID {
			return common.NewError("write_marker_validation_failed", "Write Marker is not for the same intent transaction")
		}
		if wmBlobberConnection != nil && len(txnoutput.ClientPublicKey) == 0 {
			return common.NewError("client_public_not_found", "Could not get the public key of the client")
		}
		merkleRoot := wm.MerkleRoot
		if len(wm.MerkleRoot) == 0 {
			merkleRoot = "null"
		}
		hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v", wm.DataID, merkleRoot, wm.IntentTransactionID, wm.BlobberID, wm.Timestamp, wm.ClientID)
		signatureHash := encryption.Hash(hashData)
		Logger.Info("Computed the hash for verifying wm signature. ", zap.String("hashdata", hashData), zap.String("hash", signatureHash))
		sigOK, err := encryption.Verify(txnoutput.ClientPublicKey, wm.Signature, signatureHash)
		if err != nil {
			return common.NewError("write_marker_validation_failed", "Error during verifying signature. "+err.Error())
		}
		if !sigOK {
			return common.NewError("write_marker_validation_failed", "Write marker signature is not valid")
		}
	}
	return nil
}

func (sp *StorageProtocolImpl) RedeemMarker(wm *writemarker.WriteMarkerEntity) {
	txn := transaction.NewTransactionEntity()

	sn := &transaction.CloseConnection{}
	sn.DataID = wm.WM.DataID
	sn.MerkleRoot = wm.MerkleRoot
	sn.WriteMarker = *wm.WM

	scData := &transaction.SmartContractTxnData{}
	scData.Name = transaction.CLOSE_CONNECTION_SC_NAME
	scData.InputArgs = sn

	txn.ToClientID = transaction.STORAGE_CONTRACT_ADDRESS
	txn.Value = 0
	txn.TransactionType = transaction.TxnTypeSmartContract
	txnBytes, err := json.Marshal(scData)
	if err != nil {
		Logger.Error("Error encoding sc input", zap.String("err:", err.Error()), zap.Any("scdata", scData))
		wm.Status = writemarker.Failed
		wm.StatusMessage = "Error encoding sc input. " + err.Error()
		wm.ReedeemRetries++
		wm.Write(common.GetRootContext())
		return
	}
	txn.TransactionData = string(txnBytes)

	err = txn.ComputeHashAndSign()
	if err != nil {
		Logger.Error("Signing Failed during sending close connection to the miner. ", zap.String("err:", err.Error()))
		wm.Status = writemarker.Failed
		wm.StatusMessage = "Signing Failed during sending close connection to the miner. " + err.Error()
		wm.ReedeemRetries++
		wm.Write(common.GetRootContext())
		return
	}
	transaction.SendTransactionSync(txn, sp.ServerChain)
	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
	t, err := transaction.VerifyTransaction(txn.Hash, sp.ServerChain)
	if err != nil {
		Logger.Error("Error verifying the commit transaction", zap.String("err:", err.Error()))
		wm.Status = writemarker.Failed
		wm.StatusMessage = "Signing Failed during sending close connection to the miner. " + err.Error()
		wm.ReedeemRetries++
		wm.Write(common.GetRootContext())
		return
	}
	wm.Status = writemarker.Committed
	wm.StatusMessage = t.TransactionOutput
	wm.Write(common.GetRootContext())

	debugEntity := writemarker.Provider()
	badgerdbstore.GetStorageProvider().Read(common.GetRootContext(), wm.GetKey(), debugEntity)
	Logger.Info("Debugging to see if saving was successful", zap.Any("wm", debugEntity))
	return
}
