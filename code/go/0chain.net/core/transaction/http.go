package transaction

import (
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/gosdk/core/client"
)

const TXN_SUBMIT_URL = "v1/transaction/put"
const TXN_VERIFY_URL = "v1/transaction/get/confirmation?hash="
const SC_REST_API_URL = "v1/screst/"
const REGISTER_CLIENT = "v1/client/put"

const (
	SLEEP_FOR_TXN_CONFIRMATION = 1
)

var ErrNoTxnDetail = common.NewError("missing_transaction_detail", "No transaction detail was found on any of the sharders")
var MakeSCRestAPICall func(scAddress string, relativePath string, params map[string]string, options ...string) ([]byte, error) = MakeSCRestAPICallNoHandler

func MakeSCRestAPICallNoHandler(address string, path string, params map[string]string, options ...string) ([]byte, error) {
	return client.MakeSCRestAPICall(address, path, params, options...)
}
