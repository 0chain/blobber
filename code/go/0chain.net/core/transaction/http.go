package transaction

import (
	"bytes"
	"context"
	"hash/fnv"
	"math"

	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"

	"github.com/0chain/errors"
	"github.com/0chain/gosdk/core/resty"
	"github.com/0chain/gosdk/core/util"
	"github.com/0chain/gosdk/zcncore"
)

const TXN_SUBMIT_URL = "v1/transaction/put"
const TXN_VERIFY_URL = "v1/transaction/get/confirmation?hash="
const SC_REST_API_URL = "v1/screst/"
const REGISTER_CLIENT = "v1/client/put"

const (
	SLEEP_FOR_TXN_CONFIRMATION = 1
)

var ErrNoTxnDetail = common.NewError("missing_transaction_detail", "No transaction detail was found on any of the sharders")
var MakeSCRestAPICall func(scAddress string, relativePath string, params map[string]string, chain *chain.Chain) ([]byte, error) = makeSCRestAPICall

type SCRestAPIHandler func(response map[string][]byte, numSharders int, err error)

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
}

// VerifyTransactionWithNonce verifies a transaction with known nonce.
func VerifyTransactionWithNonce(txnHash string, nonce int64) (*Transaction, error) {
	txn, err := NewTransactionEntity()
	if err != nil {
		return nil, err
	}

	txn.Hash = txnHash
	_ = txn.zcntxn.SetTransactionNonce(nonce)

	err = txn.Verify()
	if err != nil {
		return nil, err
	}
	return txn, nil
}

// MakeSCRestAPICall execute api reqeust from sharders, and parse and return result
func makeSCRestAPICall(scAddress string, relativePath string, params map[string]string, chain *chain.Chain) ([]byte, error) {
	var resMaxCounterBody []byte

	var hashMaxCounter int

	network := zcncore.GetNetwork()
	numSharders := len(network.Sharders)

	if numSharders == 0 {
		return nil, ErrNoAvailableSharder
	}

	minNumConfirmation := int(math.Ceil(float64(MinConfirmation*numSharders) / 100))

	rand := util.NewRand(numSharders)

	selectedSharders := make([]string, 0, minNumConfirmation+1)

	// random pick minNumConfirmation+1 first
	for i := 0; i <= minNumConfirmation; i++ {
		n, err := rand.Next()

		if err != nil {
			break
		}
		selectedSharders = append(selectedSharders, network.Sharders[n])
	}

	urls := make([]string, 0, len(network.Sharders))

	q := url.Values{}
	for k, v := range params {
		q.Add(k, v)
	}

	for _, sharder := range selectedSharders {
		u := fmt.Sprintf("%v/%v%v%v", sharder, SC_REST_API_URL, scAddress, relativePath)

		urls = append(urls, u+"?"+q.Encode())
	}
	logging.Logger.Info("sharder", zap.Any("URL", urls))
	header := map[string]string{
		"Content-Type":                "application/json; charset=utf-8",
		"Access-Control-Allow-Origin": "*",
	}

	//leave first item for ErrTooLessConfirmation
	var msgList = make([]string, 1, numSharders)

	r := resty.New(resty.WithHeader(header)).Then(func(req *http.Request, resp *http.Response, respBody []byte, cancelFunc context.CancelFunc, err error) error {
		if err != nil { //network issue
			msgList = append(msgList, err.Error())
			return err
		}

		url := req.URL.String()

		if resp.StatusCode != http.StatusOK {
			errorMsg := "[sharder]" + resp.Status + ": " + url
			msgList = append(msgList, errorMsg)

			return errors.Throw(ErrBadRequest, errorMsg)
		}

		hash := fnv.New32() //use fnv for better performance

		teeReader := io.TeeReader(bytes.NewReader(respBody), hash)
		resBody, err := io.ReadAll(teeReader)

		if err != nil {
			errorMsg := "[sharder]body: " + url + " " + err.Error()
			msgList = append(msgList, errorMsg)
			return errors.Throw(ErrBadRequest, errorMsg)
		}

		// NOTE: This would only return the last response, and no consensus is
		// actually met. This can be a workaround for the fix. But actually
		// it's hard to have consensus as the sharders could be in different
		// LFB when they receive the request, which means they would give
		// different response.
		resMaxCounterBody = resBody
		hashMaxCounter++

		return nil
	})

	for {
		r.DoGet(context.TODO(), urls...)

		r.Wait()

		if hashMaxCounter >= minNumConfirmation {
			break
		}

		// pick more one sharder to query transaction
		n, err := rand.Next()

		if errors.Is(err, util.ErrNoItem) {
			break
		}

		urls = []string{
			fmt.Sprintf("%v/%v%v%v", network.Sharders[n], SC_REST_API_URL, scAddress, relativePath) + "?" + q.Encode(),
		}
	}

	if hashMaxCounter < minNumConfirmation {
		msgList[0] = fmt.Sprintf("min_confirmation is %v%%, but got %v/%v sharders", MinConfirmation, hashMaxCounter, numSharders)

		return nil, errors.Throw(ErrTooLessConfirmation, msgList...)
	}

	return resMaxCounterBody, nil
}
