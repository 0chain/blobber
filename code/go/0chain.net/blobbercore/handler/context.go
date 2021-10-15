package handler

import (
	"context"
	"encoding/json"

	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/models"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/errors"
	"github.com/0chain/gosdk/constants"
	"github.com/gorilla/mux"
)

// Context api context
type Context struct {
	context.Context

	// ClientID client wallet id
	ClientID string
	// ClientKey client wallet public key
	ClientKey string
	// AllocationTx optional. allcation id in request
	AllocationTx string
	// Signature optional. signature in request
	Signature string

	Allocation *models.Allocation

	Store   datastore.Store
	Request *http.Request

	StatusCode int
}

func WithJSON(handler func(ctx *Context) (interface{}, error)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // CORS for all.
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Accept-Encoding")
			return
		}

		ctx, err := WithContext(r)

		var result interface{}
		if err == nil {
			result, err = handler(ctx)
		}

		statusCode := ctx.StatusCode

		w.Header().Set("Access-Control-Allow-Origin", "*") // CORS for all.
		w.Header().Set("Content-Type", "application/json")

		if err != nil {
			if statusCode == 0 {
				statusCode = http.StatusInternalServerError
			}

			buf, _ := json.Marshal(err)
			http.Error(w, string(buf), statusCode)
			return
		}

		if statusCode == 0 {
			statusCode = http.StatusOK
		}
		w.WriteHeader(http.StatusOK)

		if result != nil {
			json.NewEncoder(w).Encode(result) //nolint
		}

	}
}

func WithContext(r *http.Request) (*Context, error) {

	ctx := &Context{
		Context: context.TODO(),
		Request: r,
		Store:   datastore.GetStore(),
	}

	var vars = mux.Vars(r)

	ctx.ClientID = r.Header.Get(common.ClientHeader)
	ctx.ClientKey = r.Header.Get(common.ClientKeyHeader)
	ctx.AllocationTx = vars["allocation"]
	ctx.Signature = r.Header.Get(common.ClientSignatureHeader)

	if len(ctx.AllocationTx) > 0 {
		alloc, err := allocation.GetOrCreate(ctx, ctx.Store, ctx.AllocationTx)

		if err != nil {
			if errors.Is(constants.ErrBadRequest, err) {
				ctx.StatusCode = http.StatusBadRequest

			} else {
				ctx.StatusCode = http.StatusInternalServerError
			}

			return ctx, err
		}

		ctx.Allocation = alloc

		valid, err := verifySignatureFromRequest(ctx.AllocationTx, ctx.Signature, alloc.OwnerPublicKey)

		if !valid {
			ctx.StatusCode = http.StatusBadRequest
			return ctx, errors.Throw(constants.ErrBadRequest, "invalid signature "+ctx.Signature)
		}

		if err != nil {
			ctx.StatusCode = http.StatusInternalServerError
			return ctx, errors.ThrowLog(err.Error(), constants.ErrInternal, "invalid signature "+ctx.Signature)
		}
	}

	return ctx, nil
}
