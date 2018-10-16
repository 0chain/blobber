package transaction

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

	"go.uber.org/zap"
)

const TXN_SUBMIT_URL = "v1/transaction/put"
const TXN_VERIFY_URL = "v1/transaction/get/confirmation?hash="
const REGISTER_CLIENT = "v1/client/put"
const MAX_TXN_RETRIES = 3

var ErrNoTxnDetail = common.NewError("missing_transaction_detail", "No transaction detail was found on any of the sharders")

func SendTransaction(txn *Transaction, chain *chain.Chain) {
	// Get miners
	miners := chain.Miners.GetRandomNodes(chain.Miners.Size())
	for _, miner := range miners {
		url := fmt.Sprintf("%v/%v", miner.GetURLBase(), TXN_SUBMIT_URL)
		go sendTransactionToURL(url, txn)
	}
}

func SendPostRequestSync(relativeURL string, data []byte, chain *chain.Chain) {
	// Get miners
	miners := chain.Miners.GetRandomNodes(chain.Miners.Size())
	for _, miner := range miners {
		url := fmt.Sprintf("%v/%v", miner.GetURLBase(), relativeURL)
		sendPostRequest(url, data)
	}
}

func SendPostRequestAsync(relativeURL string, data []byte, chain *chain.Chain) {
	// Get miners
	miners := chain.Miners.GetRandomNodes(chain.Miners.Size())
	for _, miner := range miners {
		url := fmt.Sprintf("%v/%v", miner.GetURLBase(), relativeURL)
		go sendPostRequest(url, data)
	}
}
func sendPostRequest(url string, data []byte) ([]byte, error) {
	req, ctx, cncl, err := NewHTTPRequest(http.MethodPost, url, data)
	defer cncl()
	var resp *http.Response
	for i := 0; i < MAX_TXN_RETRIES; i++ {
		resp, err = http.DefaultClient.Do(req.WithContext(ctx))
		if err == nil {
			break
		}
		//TODO: Handle ctx cncl
		Logger.Error("SendPostRequest Error", zap.String("error", err.Error()), zap.String("URL", url))
		return nil, err
	}
	defer resp.Body.Close()
	if err == nil {
		body, _ := ioutil.ReadAll(resp.Body)
		Logger.Info("SendPostRequest success", zap.Any("response", string(body)))
		return body, nil
	}
	Logger.Error("Failed after multiple retries", zap.Int("retried", MAX_TXN_RETRIES))
	return nil, err
}

func sendTransactionToURL(url string, txn *Transaction) ([]byte, error) {
	jsObj, err := json.Marshal(txn)
	if err != nil {
		Logger.Error("Error in serializing the transaction", zap.String("error", err.Error()), zap.Any("transaction", txn))
		return nil, err
	}

	return sendPostRequest(url, jsObj)
}

func VerifyTransaction(txnHash string, chain *chain.Chain) (*Transaction, error) {
	numSharders := chain.Sharders.Size()
	numSuccess := 0
	var retTxn *Transaction
	// Get sharders
	sharders := chain.Sharders.GetRandomNodes(numSharders)
	for _, sharder := range sharders {
		url := fmt.Sprintf("%v/%v%v", sharder.GetURLBase(), TXN_VERIFY_URL, txnHash)

		response, err := http.Get(url)
		if err != nil {
			Logger.Error("Error getting transaction confirmation", zap.Any("error", err))
		} else {
			if response.StatusCode != 200 {
				continue
			}
			defer response.Body.Close()
			contents, err := ioutil.ReadAll(response.Body)
			if err != nil {
				Logger.Error("Error reading response from transaction confirmation", zap.Any("error", err))
				continue
			}
			var objmap map[string]*json.RawMessage
			err = json.Unmarshal(contents, &objmap)
			if err != nil {
				Logger.Error("Error unmarshalling response", zap.Any("error", err))
				continue
			}
			if *objmap["txn"] == nil {
				Logger.Error("Not transaction information. Only block summary.")
			}
			txn := &Transaction{}
			err = json.Unmarshal(*objmap["txn"], &txn)
			if err != nil {
				Logger.Error("Error unmarshalling to get transaction response", zap.Any("error", err))
			}
			if len(txn.Signature) > 0 {
				retTxn = txn
			}

			numSuccess++
		}
	}
	if float64(numSuccess*1.0/numSharders) > float64(0.5) {
		if retTxn != nil {
			return retTxn, nil
		}
		return nil, ErrNoTxnDetail
	}
	return nil, common.NewError("transaction_not_found", "Transaction was not found on any of the sharders")
}

func NewHTTPRequest(method string, url string, data []byte) (*http.Request, context.Context, context.CancelFunc, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Access-Control-Allow-Origin", "*")
	ctx, cncl := context.WithTimeout(context.Background(), time.Second*10)
	return req, ctx, cncl, err
}
