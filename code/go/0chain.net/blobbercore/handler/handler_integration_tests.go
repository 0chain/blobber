//go:build integration_tests
// +build integration_tests

package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/blobber/code/go/0chain.net/conductor/conductrpc"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

/* SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	r.HandleFunc("/_blobber_info", RateLimitByCommmitRL(common.ToJSONResponse(GetBlobberInfo)))

	s := r.NewRoute().Subrouter()
	setupHandlers(s)

	s.HandleFunc("/v1/file/list/{allocation}",
		RateLimitByObjectRL(common.ToJSONOrNotResponse(WithConnectionNotRespond(ListHandler)))).
		Methods(http.MethodGet, http.MethodOptions)
	s.HandleFunc("/v1/file/upload/{allocation}",
		RateLimitByFileRL(common.ToJSONOrNotResponse(WithConnectionNotRespond(UploadHandler))))
	s.HandleFunc("/v1/file/download/{allocation}",
		RateLimitByFileRL(ToByteStreamOrNot(WithConnectionNotRespond(DownloadHandler)))).
		Methods(http.MethodGet, http.MethodOptions)
}

func WithReadOnlyConnectionNotRespond(handler common.JSONResponderOrNotF) common.JSONResponderOrNotF {
	return func(ctx context.Context, r *http.Request) (interface{}, error, bool) {
		ctx = GetMetaDataStore().CreateTransaction(ctx)
		res, err, shouldRespond := handler(ctx, r)
		defer func() {
			GetMetaDataStore().GetTransaction(ctx).Rollback()
		}()
		return res, err, shouldRespond
	}
}

func WithConnectionNotRespond(handler common.JSONResponderOrNotF) common.JSONResponderOrNotF {
	return func(ctx context.Context, r *http.Request) (resp interface{}, err error, shouldRespond bool) {
		ctx = GetMetaDataStore().CreateTransaction(ctx)
		resp, err, shouldRespond = handler(ctx, r)

		defer func() {
			if err != nil {
				var rollErr = GetMetaDataStore().GetTransaction(ctx).
					Rollback().Error
				if rollErr != nil {
					Logger.Error("couldn't rollback", zap.Error(err))
				}
			}
		}()

		if err != nil {
			Logger.Error("Error in handling the request." + err.Error())
			return
		}
		err = GetMetaDataStore().GetTransaction(ctx).Commit().Error
		if err != nil {
			return resp, common.NewErrorf("commit_error",
				"error committing to meta store: %v", err), true
		}
		return
	}
}

func ToByteStreamOrNot(handler common.JSONResponderOrNotF) common.ReqRespHandlerf {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		data, err, shouldRespond := handler(ctx, r)

		if !shouldRespond {
			return
		}

		if err != nil {
			if cerr, ok := err.(*common.Error); ok {
				w.Header().Set(common.AppErrorHeader, cerr.Code)
			}
			if data != nil {
				responseString, _ := json.Marshal(data)
				http.Error(w, string(responseString), 400)
			} else {
				http.Error(w, err.Error(), 400)
			}
		} else if data != nil {
			rawdata, ok := data.([]byte)
			if ok {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Write(rawdata) //nolint:errcheck
			} else {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(data) //nolint:errcheck
			}
		}
	}
}

/*ListHandler is the handler to respond to list requests fro clients*/
func ListHandler(ctx context.Context, r *http.Request) (interface{}, error, bool) {
	state := conductrpc.Client().State()

	if state.BlobberList.Adversarial == node.Self.ID && state.BlobberList.SendWrongData {
		var result blobberhttp.ListResult
		return result, nil, true
	}

	if state.BlobberList.Adversarial == node.Self.ID && state.BlobberList.SendWrongMetadata {
		listResult, err := listHandler(ctx, r)

		var result *blobberhttp.ListResult

		result = listResult.(*blobberhttp.ListResult)

		result.Meta = make(map[string]interface{})
		result.Meta["type"] = ""

		return result, err, true
	}

	if state.BlobberList.Adversarial == node.Self.ID && state.BlobberList.NotRespond {
		return nil, nil, false
	}

	if state.BlobberList.Adversarial == node.Self.ID && state.BlobberList.ReturnError {
		return nil, common.NewError("list_file", "adversarial"), true
	}

	result, err := listHandler(ctx, r)

	return result, err, true
}

/*DownloadHandler is the handler to respond to download requests from clients*/
func DownloadHandler(ctx context.Context, r *http.Request) (interface{}, error, bool) {
	state := conductrpc.Client().State()

	if state.BlobberDownload.Adversarial == node.Self.ID && state.BlobberDownload.NotRespond {
		return nil, nil, false
	}

	if state.BlobberDownload.Adversarial == node.Self.ID && state.BlobberDownload.ReturnError {
		return nil, common.NewError("download_file", "adversarial"), true
	}

	result, err := downloadHandler(ctx, r)

	return result, err, true
}

/*uploadHandler is the handler to respond to upload requests fro clients*/
func UploadHandler(ctx context.Context, r *http.Request) (interface{}, error, bool) {
	state := conductrpc.Client().State()

	if state.BlobberUpload.Adversarial == node.Self.ID && state.BlobberUpload.NotRespond && (r.Method == "PUT" || r.Method == "POST") {
		return nil, nil, false
	}

	if state.BlobberDelete.Adversarial == node.Self.ID && state.BlobberDelete.NotRespond && r.Method == "DELETE" {
		return nil, nil, false
	}

	if state.BlobberUpload.Adversarial == node.Self.ID && state.BlobberUpload.ReturnError && (r.Method == "PUT" || r.Method == "POST") {
		return nil, common.NewError("upload_file", "adversarial"), true
	}

	if state.BlobberDelete.Adversarial == node.Self.ID && state.BlobberDelete.ReturnError && r.Method == "DELETE" {
		return nil, common.NewError("delete_file", "adversarial"), true
	}

	result, err := uploadHandler(ctx, r)
	return result, err, true
}
