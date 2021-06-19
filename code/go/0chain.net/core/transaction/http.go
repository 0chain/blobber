package transaction

import (
	"crypto/sha1"
	"encoding/hex"

	//"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"

	//"sync"
	"time"

	"0chain.net/core/chain"
	"0chain.net/core/common"
	. "0chain.net/core/logging"
	"github.com/0chain/gosdk/core/util"
	"github.com/0chain/gosdk/zcncore"

	//"0chain.net/core/util"

	"go.uber.org/zap"
)

const TXN_SUBMIT_URL = "v1/transaction/put"
const TXN_VERIFY_URL = "v1/transaction/get/confirmation?hash="
const SC_REST_API_URL = "v1/screst/"
const REGISTER_CLIENT = "v1/client/put"

const (
	SLEEP_FOR_TXN_CONFIRMATION = 5
	SC_REST_API_ATTEMPTS       = 3
)

var ErrNoTxnDetail = common.NewError("missing_transaction_detail", "No transaction detail was found on any of the sharders")

// func SendTransaction(txn *Transaction, chain *chain.Chain) {
// 	// Get miners
// 	miners := chain.Miners.GetRandomNodes(chain.Miners.Size())
// 	for _, miner := range miners {
// 		url := fmt.Sprintf("%v/%v", miner.GetURLBase(), TXN_SUBMIT_URL)
// 		go sendTransactionToURL(url, txn, nil)
// 	}
// }

type SCRestAPIHandler func(response map[string][]byte, numSharders int, err error)

// func SendTransactionSync(txn *Transaction, chain *chain.Chain) {
// 	wg := sync.WaitGroup{}
// 	wg.Add(chain.Miners.Size())
// 	// Get miners
// 	miners := chain.Miners.GetRandomNodes(chain.Miners.Size())
// 	for _, miner := range miners {
// 		url := fmt.Sprintf("%v/%v", miner.GetURLBase(), TXN_SUBMIT_URL)
// 		go sendTransactionToURL(url, txn, &wg)
// 	}
// 	wg.Wait()
// }

// func SendPostRequestSync(relativeURL string, data []byte, chain *chain.Chain) {
// 	wg := sync.WaitGroup{}
// 	wg.Add(chain.Miners.Size())
// 	// Get miners
// 	miners := chain.Miners.GetRandomNodes(chain.Miners.Size())
// 	for _, miner := range miners {
// 		url := fmt.Sprintf("%v/%v", miner.GetURLBase(), relativeURL)
// 		go util.SendPostRequest(url, data, &wg)
// 	}
// 	wg.Wait()
// }

// func SendPostRequestAsync(relativeURL string, data []byte, chain *chain.Chain) {
// 	// Get miners
// 	miners := chain.Miners.GetRandomNodes(chain.Miners.Size())
// 	for _, miner := range miners {
// 		url := fmt.Sprintf("%v/%v", miner.GetURLBase(), relativeURL)
// 		go util.SendPostRequest(url, data, nil)
// 	}
// }

// func sendTransactionToURL(url string, txn *Transaction, wg *sync.WaitGroup) ([]byte, error) {
// 	if wg != nil {
// 		defer wg.Done()
// 	}
// 	jsObj, err := json.Marshal(txn)
// 	if err != nil {
// 		Logger.Error("Error in serializing the transaction", zap.String("error", err.Error()), zap.Any("transaction", txn))
// 		return nil, err
// 	}

// 	return util.SendPostRequest(url, jsObj, nil)
// }

func VerifyTransaction(txnHash string, chain *chain.Chain) (*Transaction, error) {
	txn, err := NewTransactionEntity()
	if err != nil {
		return nil, err
	}

	txn.Hash = txnHash
	err = txn.Verify()
	if err != nil {
		return nil, err
	}
	return txn, nil

	// numSharders := chain.Sharders.Size()
	// numSuccess := 0
	// var retTxn *Transaction
	// // Get sharders
	// sharders := chain.Sharders.GetRandomNodes(numSharders)
	// for _, sharder := range sharders {
	// 	url := fmt.Sprintf("%v/%v%v", sharder.GetURLBase(), TXN_VERIFY_URL, txnHash)
	// 	var netTransport = &http.Transport{
	// 		Dial: (&net.Dialer{
	// 			Timeout: 5 * time.Second,
	// 		}).Dial,
	// 		TLSHandshakeTimeout: 5 * time.Second,
	// 	}
	// 	var netClient = &http.Client{
	// 		Timeout:   time.Second * 10,
	// 		Transport: netTransport,
	// 	}
	// 	resp, err := netClient.Get(url)
	// 	if err != nil {
	// 		Logger.Error("Error getting transaction confirmation", zap.Any("error", err))
	// 		numSharders--
	// 	} else {
	// 		if resp.StatusCode != 200 {
	// 			continue
	// 		}
	// 		defer resp.Body.Close()
	// 		contents, err := ioutil.ReadAll(resp.Body)
	// 		if err != nil {
	// 			Logger.Error("Error reading response from transaction confirmation", zap.Any("error", err))
	// 			continue
	// 		}
	// 		var objmap map[string]json.RawMessage
	// 		err = json.Unmarshal(contents, &objmap)
	// 		if err != nil {
	// 			Logger.Error("Error unmarshalling response", zap.Any("error", err))
	// 			continue
	// 		}
	// 		if _, ok := objmap["txn"]; !ok {
	// 			Logger.Info("Not transaction information. Only block summary.", zap.Any("sharder", url), zap.Any("output", string(contents)))
	// 			if _, ok := objmap["block_hash"]; ok {
	// 				numSuccess++
	// 				continue
	// 			}
	// 			Logger.Info("Sharder does not have the block summary", zap.Any("sharder", url), zap.Any("output", string(contents)))
	// 			continue
	// 		}
	// 		txn := &Transaction{}
	// 		err = json.Unmarshal(objmap["txn"], txn)
	// 		if err != nil {
	// 			Logger.Error("Error unmarshalling to get transaction response", zap.Any("error", err))
	// 		}
	// 		if len(txn.Signature) > 0 {
	// 			retTxn = txn
	// 		}

	// 		numSuccess++
	// 	}
	// }
	// if numSharders == 0 || float64(numSuccess*1.0/numSharders) > float64(0.5) {
	// 	if retTxn != nil {
	// 		return retTxn, nil
	// 	}
	// 	return nil, ErrNoTxnDetail
	// }
	// return nil, common.NewError("transaction_not_found", "Transaction was not found on any of the sharders")
}

func MakeSCRestAPICall(scAddress string, relativePath string, params map[string]string, chain *chain.Chain, handler SCRestAPIHandler) ([]byte, error) {
	var resMaxCounterBody []byte
	resBodies := make(map[string][]byte)

	var hashMaxCounter int
	hashCounters := make(map[string]int)

	network := zcncore.GetNetwork()
	numSharders := len(network.Sharders)
	sharders := util.GetRandom(network.Sharders, numSharders)

	for _, sharder := range sharders {
		// Make one or more requests (in case of unavailability, see 503/504 errors)
		var err error
		var resp *http.Response
		var counter int = SC_REST_API_ATTEMPTS

		netTransport := &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 5 * time.Second,
		}

		netClient := &http.Client{
			Timeout:   10 * time.Second,
			Transport: netTransport,
		}

		uString := fmt.Sprintf("%v/%v%v%v", sharder, SC_REST_API_URL, scAddress, relativePath)
		fmt.Println(uString)
		u, _ := url.Parse(uString)
		q := u.Query()
		for k, v := range params {
			q.Add(k, v)
		}
		u.RawQuery = q.Encode()

		for counter > 0 {
			resp, err = netClient.Get(u.String())
			if err != nil {
				break
			}

			// if it's not available, retry if there are any retry attempts
			if resp.StatusCode == 503 || resp.StatusCode == 504 {
				resp.Body.Close()
				counter--
			} else {
				break
			}
		}

		if err != nil {
			Logger.Error("Error getting response for sc rest api", zap.Any("error", err), zap.Any("sharder_url", sharder))
			numSharders--
		} else {
			if resp.StatusCode != 200 {
				resBody, _ := ioutil.ReadAll(resp.Body)
				Logger.Error("Got error response from sc rest api", zap.Any("response", string(resBody)))
				resp.Body.Close()
				continue
			}

			defer resp.Body.Close() // TODO: is it really needed here? or put it above and drop other "Body.Close"s

			hash := sha1.New()
			teeReader := io.TeeReader(resp.Body, hash)
			resBody, err := ioutil.ReadAll(teeReader)

			if err != nil {
				Logger.Error("Error reading response", zap.Any("error", err))
				resp.Body.Close()
				continue
			}

			hashString := hex.EncodeToString(hash.Sum(nil))
			hashCounters[hashString]++

			if hashCounters[hashString] > hashMaxCounter {
				hashMaxCounter = hashCounters[hashString]
				resMaxCounterBody = resBody
			}

			resBodies[sharder] = resMaxCounterBody // TODO: check it! looks suspicious. assigned value is not set for some interations. maybe should be = resBody?
			resp.Body.Close()
		}
	}

	var err error

	// is it less than or equal to 50%
	if hashMaxCounter <= (numSharders / 2) {
		err = common.NewError("invalid_response", "Sharder responses were invalid. Hash mismatch")
	}

	if handler != nil {
		handler(resBodies, numSharders, err)
	}

	// is it more than 50%
	if hashMaxCounter > (numSharders / 2) {
		return resMaxCounterBody, nil
	}

	return nil, err
}
