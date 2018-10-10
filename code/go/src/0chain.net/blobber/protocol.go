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
	"0chain.net/config"
	. "0chain.net/logging"
	"0chain.net/node"
	"go.uber.org/zap"
)

const MAX_REGISTRATION_RETRIES = 3

// TODO: (0) Fix hardcoding
const STORAGE_CONTRACT_ADDRESS 	= "6dba10422e368813802877a85039d3985d96760ed844092319743fb3a76712d7"
const PUT_TRANSACTION 			= "v1/transaction/put"
var   REGISTRATION_DATA			= json.RawMessage(`{"add_blobber": ""}`)
const SMART_CONTRACT_TYPE		= 1000


// Storage smart contract 
type StorageNode struct {
	ID        string `json:"id"`
	BaseURL   string `json:"url"`
	PublicKey string `json:"-"`
}

//StorageProtocol - interface for the storage protocol
type StorageProtocol interface {
	Register()
	VerifyAllocationTransaction()
	VerifyBlobberTransaction()
	VerifyMarker()
	CollectMarker()
	RedeemMarker()
}

//StorageProtocolImpl - implementation of the storage protocol
type StorageProtocolImpl struct {
	ServerChain *chain.Chain
}

//ProtocolImpl - singleton for the protocol implementation
var ProtocolImpl StorageProtocol

func GetProtocolImpl() StorageProtocol {
	return ProtocolImpl
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
			ChainID 	: 	config.Configuration.ChainID,
			Value		: 	0,
			Data		:   REGISTRATION_DATA,
			TxType 		:   SMART_CONTRACT_TYPE,
			CreationDate: 	common.Now(),
		}
		hashdata := fmt.Sprintf("%v:%v:%v:%v:%v", txn.CreationDate, txn.ClientID, 
					txn.ToClientID, txn.Value, encryption.Hash(string(txn.Data)))
		var err error
		txn.Signature, err = node.Self.Sign(hashdata)
		if (err != nil) {
			Logger.Info("Signing Failed")
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

func (sp *StorageProtocolImpl) VerifyMarker() {

}

func (sp *StorageProtocolImpl) CollectMarker() {

}

func (sp *StorageProtocolImpl) RedeemMarker() {

}

/*============================ Private functions =======================*/
func sendTransaction(url string, txn transaction.Transaction) {
	jsObj, err := json.Marshal(txn)
	if (err != nil) {
		fmt.Println("Error:" , err)
	}
	fmt.Println("TXN:", txn)
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
		fmt.Println("response Status:", resp.Status)
		fmt.Println("response Headers:", resp.Header)
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("response Body:", string(body))			
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

