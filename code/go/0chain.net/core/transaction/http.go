package transaction

import (
	"context"
	"crypto/sha1"
	"encoding/hex"

	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"

	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/errors"
	"github.com/0chain/gosdk/core/resty"
	"github.com/0chain/gosdk/zcncore"

	"go.uber.org/zap"
)

const TXN_SUBMIT_URL = "v1/transaction/put"
const TXN_VERIFY_URL = "v1/transaction/get/confirmation?hash="
const SC_REST_API_URL = "v1/screst/"
const REGISTER_CLIENT = "v1/client/put"

const (
	SLEEP_FOR_TXN_CONFIRMATION = 5
)

var ErrNoTxnDetail = common.NewError("missing_transaction_detail", "No transaction detail was found on any of the sharders")

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

// MakeSCRestAPICall execute api reqeust from sharders, and parse and return result
func MakeSCRestAPICall(scAddress string, relativePath string, params map[string]string, chain *chain.Chain) ([]byte, error) {
	var resMaxCounterBody []byte

	var hashMaxCounter int
	hashCounters := make(map[string]int)

	network := zcncore.GetNetwork()
	numSharders := len(network.Sharders)

	if numSharders == 0 {
		return nil, ErrNoAvailableSharder
	}

	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: DefaultDialTimeout,
		}).Dial,
		TLSHandshakeTimeout: DefaultDialTimeout,
	}

	r := resty.New(transport, func(req *http.Request, resp *http.Response, cancelFunc context.CancelFunc, err error) error {

		if err != nil {
			return errors.Throw(ErrBadRequest, err.Error())
		}

		if resp.StatusCode != http.StatusOK {
			resBody, _ := ioutil.ReadAll(resp.Body)
			resp.Body.Close()

			Logger.Error("[sharder]"+resp.Status, zap.String("url", req.URL.String()), zap.String("response", string(resBody)))

			return errors.Throw(ErrBadRequest, req.URL.String()+" "+resp.Status)

		}

		hash := sha1.New()
		teeReader := io.TeeReader(resp.Body, hash)
		resBody, err := ioutil.ReadAll(teeReader)
		resp.Body.Close()

		if err != nil {
			Logger.Error("[sharder]"+resp.Status, zap.String("url", req.URL.String()), zap.String("response", string(resBody)))

			return errors.Throw(ErrBadRequest, req.URL.String()+" "+err.Error())

		}

		hashString := hex.EncodeToString(hash.Sum(nil))
		hashCounters[hashString]++

		if hashCounters[hashString] > hashMaxCounter {
			hashMaxCounter = hashCounters[hashString]
			resMaxCounterBody = resBody
		}

		consensus := int(float64(hashMaxCounter) / float64(numSharders) * 100)

		// It is confirmed, and cancel other requests for performance
		if consensus > 0 && consensus >= MinConfirmation {
			cancelFunc()
			return nil
		}

		return nil
	},
		resty.WithTimeout(DefaultRequestTimeout),
		resty.WithRetry(DefaultRetry))

	urls := make([]string, 0, len(network.Sharders))

	q := url.Values{}
	for k, v := range params {
		q.Add(k, v)
	}

	for _, sharder := range network.Sharders {

		u := fmt.Sprintf("%v/%v%v%v", sharder, SC_REST_API_URL, scAddress, relativePath)

		urls = append(urls, u+"?"+q.Encode())
	}

	r.DoGet(context.Background(), urls...)

	errs := r.Wait()

	consensus := int(float64(hashMaxCounter) / float64(numSharders) * 100)

	if consensus < MinConfirmation {
		msgList := make([]string, 0, len(errs))

		for _, msg := range errs {
			msgList = append(msgList, msg)
		}
		return errors.Throw(ErrTooLessConfirmation, msgList...)
	}

	return resMaxCounterBody, nil

}
