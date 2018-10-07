package blobber

import (
	"fmt"
	"time"
	"bytes"
	"io/ioutil"
	"encoding/json"
	"net/http"

	"0chain.net/chain"
	"0chain.net/encryption"
	"0chain.net/config"
	. "0chain.net/logging"
	"0chain.net/node"
	"go.uber.org/zap"
)

const MAX_REGISTRATION_RETRIES = 3

// TODO: (0) Fix hardcoding
const STORAGE_CONTRACT_ADDRESS 	= "6dba10422e368813802877a85039d3985d96760ed844092319743fb3a76712d7"
const PUT_TRANSACTION 			= "v1/transaction/put"

type Transaction struct {
	ClientID  			string 				`json:"client_id,omitempty"`
	PublicKey 			string        		`json:"public_key,omitempty"`
	ToClientID      	string    		 	`json:"to_client_id,omitempty"`
	ChainID         	string    			`json:"chain_id,omitempty"`
}

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
		// Get a random miner
		miner := sp.ServerChain.Miners.GetRandomNode()
		url := fmt.Sprintf("%v/%v", miner.GetURLBase(), PUT_TRANSACTION);
		// url := fmt.Sprintf("http://localhost:7071/%v", PUT_TRANSACTION);
		txn := Transaction {
			ClientID 	:	node.Self.ID,
			PublicKey 	: 	encryption.Hash(node.Self.ID),
			ToClientID 	:	STORAGE_CONTRACT_ADDRESS,
			ChainID 	: 	config.Configuration.ChainID,
		}
		// TODO: Add Signature..
		fmt.Println(txn)
		jsObj, err := json.Marshal(txn)
		if (err != nil) {
			fmt.Println("Error:" , err)
		}
		fmt.Println("JSON:", jsObj)
		Logger.Info("Registering to Miner", zap.String("ID", miner.ID), zap.String("URL", url))
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsObj))
	   	req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Access-Control-Allow-Origin", "*")		
		tr := &http.Transport{
			ResponseHeaderTimeout: 3 * time.Second,
		}
		var resp *http.Response
		client := &http.Client{Transport: tr}
		for i:= 0; i < MAX_REGISTRATION_RETRIES; i++ {
			resp, err = client.Do(req)
			if err == nil {
			    break;
			}
			Logger.Error("Register error", zap.String("error", err.Error()))
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
		// TODO: Handle error?
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
