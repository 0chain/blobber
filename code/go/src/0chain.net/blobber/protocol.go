package blobber

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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
	// Get miners
	miners := sp.ServerChain.Miners.GetRandomNodes(sp.ServerChain.Miners.Size())
	for _, miner := range miners {
		url := fmt.Sprintf("%v/%v", miner.GetURLBase(), transaction.TXN_SUBMIT_URL)
		go sendTransaction(url, txn)
	}
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

/*============================ Private functions =======================*/
func sendTransaction(url string, txn *transaction.Transaction) {
	jsObj, err := json.Marshal(txn)
	if err != nil {
		fmt.Println("Error:", err)
	}
	req, ctx, cncl, err := newHttpRequest(http.MethodPost, url, jsObj)
	defer cncl()
	var resp *http.Response
	for i := 0; i < transaction.MAX_TXN_RETRIES; i++ {
		resp, err = http.DefaultClient.Do(req.WithContext(ctx))
		if err == nil {
			break
		}
		//TODO: Handle ctx cncl
		Logger.Error("Register", zap.String("error", err.Error()), zap.String("URL", url))
	}

	if err == nil {
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("response Status:", resp.Status, "Body:", string(body))
		return
	}
	Logger.Error("Failed after ", zap.Int("retried", transaction.MAX_TXN_RETRIES))
}

func newHttpRequest(method string, url string, data []byte) (*http.Request, context.Context, context.CancelFunc, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Access-Control-Allow-Origin", "*")
	ctx, cncl := context.WithTimeout(context.Background(), time.Second*10)
	return req, ctx, cncl, err
}
