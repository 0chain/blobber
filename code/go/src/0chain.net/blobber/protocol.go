package blobber

import (
	"fmt"
	"time"
	"bytes"
	"context"
	"io/ioutil"
	"encoding/json"
	"net/http"

	"0chain.net/chain"
	"0chain.net/common"
	"0chain.net/encryption"
	"0chain.net/transaction"
	. "0chain.net/logging"
	"0chain.net/node"
	"go.uber.org/zap"

	"0chain.net/node"
	"0chain.net/writemarker"
)

const MAX_REGISTRATION_RETRIES = 3

// TODO: (0) Fix hardcoding
const STORAGE_CONTRACT_ADDRESS 	= "6dba10422e368813802877a85039d3985d96760ed844092319743fb3a76712d7"
const PUT_TRANSACTION 			= "v1/transaction/put"
const SMART_CONTRACT_TYPE		= 1000


// Storage smart contract 
type StorageData struct {
	Name 	  string 	`json:"name"`
	ID        string 	`json:"id"`
	BaseURL   string 	`json:"url"`
}



//StorageProtocol - interface for the storage protocol
type StorageProtocol interface {
	Register()
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

//ProtocolImpl - singleton for the protocol implementation
var ProtocolImpl StorageProtocol

func GetProtocolImpl(allocationID string, intentTxn string, dataID string, wm *writemarker.WriteMarker) StorageProtocol {
	return &StorageProtocolImpl{
		ServerChain:  chain.GetServerChain(),
		AllocationID: allocationID,
		IntentTxnID:  intentTxn,
		DataID:       dataID,
		WriteMarker:  wm}
}

//SetupProtocol - sets up the protocol for the chain
func SetupProtocol(c *chain.Chain) {
	ProtocolImpl = &StorageProtocolImpl{ServerChain: c}
}

func (sp *StorageProtocolImpl) Register() {
	if (sp.ServerChain != nil) {	

		txn := transaction.Transaction {
			Version 	: 	transaction.TRANSACTION_VERION,
			ClientID 	:	node.Self.ID,
			PublicKey 	: 	encryption.Hash(node.Self.ID),
			ToClientID 	:	STORAGE_CONTRACT_ADDRESS,
			Value		: 	0,
			TxType 		:   SMART_CONTRACT_TYPE,
			CreationDate: 	common.Now(),
		}
		txn.Data = fmt.Sprintf("{\"name\":\"add_blobber\",\"input\":{\"id\":\"%v\",\"url\":\"%v\"", node.Self.GetKey(),node.Self.GetURLBase())
		hashdata := fmt.Sprintf("%v:%v:%v:%v:%v", txn.CreationDate, txn.ClientID, 
					txn.ToClientID, txn.Value, encryption.Hash(txn.Data))
		txn.Hash = encryption.Hash(hashdata)
		var err error
		txn.Signature, err = node.Self.Sign(txn.Hash)
		if (err != nil) {
			Logger.Info("Signing Failed",zap.String("err:", err.Error()))
		}
		// Get miners
		miners := sp.ServerChain.Miners.GetRandomNodes(sp.ServerChain.Miners.Size())
		for _, miner := range miners {
			url := fmt.Sprintf("%v/%v", miner.GetURLBase(), PUT_TRANSACTION);
			go sendTransaction(url, txn)
    	}
	}
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
func sendTransaction(url string, txn transaction.Transaction) {
	jsObj, err := json.Marshal(txn)
	if (err != nil) {
		fmt.Println("Error:" , err)
	}
	req, ctx, cncl, err := newHttpRequest(http.MethodPost, url, jsObj)
	defer cncl()
	var resp *http.Response
	for i:= 0; i < MAX_REGISTRATION_RETRIES; i++ {
		resp, err = http.DefaultClient.Do(req.WithContext(ctx))
		if err == nil {
		    break;
		}
		//TODO: Handle ctx cncl
		Logger.Error("Register", zap.String("error", err.Error()), zap.String("URL", url))
	}

	if (err == nil) {
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("response Status:", resp.Status, "Body:", string(body))			
		return
	}
	Logger.Error("Failed after ", zap.Int("retried", MAX_REGISTRATION_RETRIES))
}

func newHttpRequest(method string, url string, data []byte) (*http.Request, context.Context, context.CancelFunc, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
   	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Access-Control-Allow-Origin", "*")
	ctx, cncl := context.WithTimeout(context.Background(), time.Second*10)
	return req, ctx, cncl, err
}

