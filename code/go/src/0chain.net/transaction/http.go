package transaction

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"0chain.net/chain"
	"0chain.net/common"
	. "0chain.net/logging"
	"0chain.net/util"

	"go.uber.org/zap"
)

const TXN_SUBMIT_URL = "v1/transaction/put"
const TXN_VERIFY_URL = "v1/transaction/get/confirmation?hash="
const SC_REST_API_URL = "v1/screst/"
const REGISTER_CLIENT = "v1/client/put"

const SLEEP_FOR_TXN_CONFIRMATION = 5

var ErrNoTxnDetail = common.NewError("missing_transaction_detail", "No transaction detail was found on any of the sharders")

func SendTransaction(txn *Transaction, chain *chain.Chain) {
	// Get miners
	miners := chain.Miners.GetRandomNodes(chain.Miners.Size())
	for _, miner := range miners {
		url := fmt.Sprintf("%v/%v", miner.GetURLBase(), TXN_SUBMIT_URL)
		go sendTransactionToURL(url, txn, nil)
	}
}

func SendTransactionSync(txn *Transaction, chain *chain.Chain) {
	wg := sync.WaitGroup{}
	wg.Add(chain.Miners.Size())
	// Get miners
	miners := chain.Miners.GetRandomNodes(chain.Miners.Size())
	for _, miner := range miners {
		url := fmt.Sprintf("%v/%v", miner.GetURLBase(), TXN_SUBMIT_URL)
		go sendTransactionToURL(url, txn, &wg)
	}
	wg.Wait()
}

func SendPostRequestSync(relativeURL string, data []byte, chain *chain.Chain) {
	wg := sync.WaitGroup{}
	wg.Add(chain.Miners.Size())
	// Get miners
	miners := chain.Miners.GetRandomNodes(chain.Miners.Size())
	for _, miner := range miners {
		url := fmt.Sprintf("%v/%v", miner.GetURLBase(), relativeURL)
		go util.SendPostRequest(url, data, &wg)
	}
	wg.Wait()
}

func SendPostRequestAsync(relativeURL string, data []byte, chain *chain.Chain) {
	// Get miners
	miners := chain.Miners.GetRandomNodes(chain.Miners.Size())
	for _, miner := range miners {
		url := fmt.Sprintf("%v/%v", miner.GetURLBase(), relativeURL)
		go util.SendPostRequest(url, data, nil)
	}
}

func sendTransactionToURL(url string, txn *Transaction, wg *sync.WaitGroup) ([]byte, error) {
	if wg != nil {
		defer wg.Done()
	}
	jsObj, err := json.Marshal(txn)
	if err != nil {
		Logger.Error("Error in serializing the transaction", zap.String("error", err.Error()), zap.Any("transaction", txn))
		return nil, err
	}

	return util.SendPostRequest(url, jsObj, nil)
}

func VerifyTransaction(txnHash string, chain *chain.Chain) (*Transaction, error) {
	numSharders := chain.Sharders.Size()
	numSuccess := 0
	var retTxn *Transaction
	// Get sharders
	sharders := chain.Sharders.GetRandomNodes(numSharders)
	for _, sharder := range sharders {
		url := fmt.Sprintf("%v/%v%v", sharder.GetURLBase(), TXN_VERIFY_URL, txnHash)
		var netTransport = &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 5 * time.Second,
		}
		var netClient = &http.Client{
			Timeout:   time.Second * 10,
			Transport: netTransport,
		}
		response, err := netClient.Get(url)
		if err != nil {
			Logger.Error("Error getting transaction confirmation", zap.Any("error", err))
			numSharders--
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
	if numSharders == 0 || float64(numSuccess*1.0/numSharders) > float64(0.5) {
		if retTxn != nil {
			return retTxn, nil
		}
		return nil, ErrNoTxnDetail
	}
	return nil, common.NewError("transaction_not_found", "Transaction was not found on any of the sharders")
}

func MakeSCRestAPICall(scAddress string, relativePath string, params map[string]string, chain *chain.Chain, entity interface{}) (interface{}, error) {
	numSharders := chain.Sharders.Size()
	sharders := chain.Sharders.GetRandomNodes(numSharders)
	responses := make(map[string]int)
	var retObj interface{}
	maxCount := 0
	for _, sharder := range sharders {
		urlString := fmt.Sprintf("%v/%v%v%v", sharder.GetURLBase(), SC_REST_API_URL, scAddress, relativePath)
		urlObj, _ := url.Parse(urlString)
		q := urlObj.Query()
		for k, v := range params {
			q.Add(k, v)
		}
		urlObj.RawQuery = q.Encode()
		h := sha1.New()
		var netTransport = &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 5 * time.Second,
		}
		var netClient = &http.Client{
			Timeout:   time.Second * 10,
			Transport: netTransport,
		}
		response, err := netClient.Get(urlObj.String())
		if err != nil {
			Logger.Error("Error getting response for sc rest api", zap.Any("error", err))
			numSharders--
		} else {
			if response.StatusCode != 200 {
				continue
			}
			defer response.Body.Close()
			tReader := io.TeeReader(response.Body, h)
			d := json.NewDecoder(tReader)
			d.UseNumber()
			err := d.Decode(entity)
			if err != nil {
				Logger.Error("Error unmarshalling response", zap.Any("error", err))
				continue
			}
			hashBytes := h.Sum(nil)
			hash := hex.EncodeToString(hashBytes)
			responses[hash]++
			if responses[hash] > maxCount {
				maxCount = responses[hash]
				retObj = entity
			}
		}
	}
	if maxCount > (numSharders / 2) {
		return retObj, nil
	}
	return nil, common.NewError("invalid_response", "Sharder responses were invalid. Hash mismatch")
}
