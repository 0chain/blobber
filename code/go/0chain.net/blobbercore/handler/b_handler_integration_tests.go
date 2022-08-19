//go:build integration_tests
// +build integration_tests

package handler

import (
	"context"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/blobber/code/go/0chain.net/conductor/conductrpc"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/gorilla/mux"
)

/* SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	setupHandlers(r)

	r.HandleFunc("/v1/file/list/{allocation}",
		RateLimitByObjectRL(common.ToJSONResponse(WithReadOnlyConnection(ListHandler)))).
		Methods(http.MethodGet, http.MethodOptions)
}

/*JSONResponderOrNotF - a handler that takes standard request (non-json) and responds with a json response
* Useful for POST opertaion where the input is posted as json with
*    Content-type: application/json
* header
* For test purposes it is useful to not respond
 */
type JSONResponderOrNotF func(ctx context.Context, r *http.Request) (interface{}, error, bool)

/* ToJSONOrNotResponse - An adapter that takes a handler of the form
* func AHandler(r *http.Request) (interface{}, error)
* which takes a request object, processes and returns an object or an error
* and converts into a standard request/response handler or simply does not respond
* for test purposes
 */
func ToJSONOrNotResponse(handler JSONResponderOrNotF) common.ReqRespHandlerf {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // CORS for all.
		if r.Method == "OPTIONS" {
			common.SetupCORSResponse(w, r)
			return
		}
		ctx := r.Context()
		data, err, shouldRespond := handler(ctx, r)

		if shouldRespond {
			common.Respond(w, data, err)
		}
	}
}

func WithReadOnlyConnectionNotRespond(handler JSONResponderOrNotF) JSONResponderOrNotF {
	return func(ctx context.Context, r *http.Request) (interface{}, error, bool) {
		ctx = GetMetaDataStore().CreateTransaction(ctx)
		res, err, shouldRespond := handler(ctx, r)
		defer func() {
			GetMetaDataStore().GetTransaction(ctx).Rollback()
		}()
		return res, err, shouldRespond
	}
}

/*ListHandler is the handler to respond to list requests fro clients*/
func ListHandler(ctx context.Context, r *http.Request) (interface{}, error, bool) {
	state := conductrpc.Client().State()

	if state.BlobberList.Adversarial == node.Self.ID && state.BlobberList.SendWrongData {
		var result blobberhttp.ListResult
		return result, nil, true
	} else if state.BlobberList.Adversarial == node.Self.ID && state.BlobberList.SendWrongMetadata {
		listResult, err := listHandler(ctx, r)

		var result *blobberhttp.ListResult

		result = listResult.(*blobberhttp.ListResult)

		result.Meta = make(map[string]interface{})
		result.Meta["type"] = ""

		return result, err, true
	} else if state.BlobberList.Adversarial == node.Self.ID && state.BlobberList.NotRespond {
		return nil, nil, false
	}

	result, err := listHandler(ctx, r)

	return result, err, true
}
