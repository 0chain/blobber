package handler

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/errors"
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

	Allocation *allocation.Allocation

	// Vars route variables
	Vars map[string]string

	Store   datastore.Store
	Request *http.Request

	StatusCode int
}

func (c *Context) Var(key string) string {
	if c == nil || c.Vars == nil {
		return ""
	}

	return c.Vars[key]
}

// FormValue get value from form data
func (c *Context) FormValue(key string) string {
	if c == nil || c.Vars == nil {
		return ""
	}
	return c.Request.FormValue(key)
}

// FormTime get time from form data
func (c *Context) FormTime(key string) *time.Time {
	if c == nil || c.Vars == nil {
		return nil
	}
	value := c.Request.FormValue(key)
	if len(value) == 0 {
		return nil
	}

	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil
	}

	t := time.Unix(seconds, 0)
	return &t
}

type ErrorResponse struct {
	Error string
}

// WithHandler process handler to respond request
func WithHandler(handler func(ctx *Context) (interface{}, error)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // CORS for all.
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "*")
			return
		}

		TryParseForm(r)

		w.Header().Set("Access-Control-Allow-Origin", "*") // CORS for all.
		w.Header().Set("Content-Type", "application/json")

		ctx, err := WithAuth(r)
		statusCode := ctx.StatusCode

		if err != nil {
			if statusCode == 0 {
				statusCode = http.StatusInternalServerError
			}

			http.Error(w, err.Error(), statusCode)
			return
		}

		result, err := handler(ctx)
		statusCode = ctx.StatusCode

		if err != nil {
			if statusCode == 0 {
				statusCode = http.StatusInternalServerError
			}

			http.Error(w, err.Error(), statusCode)
			return
		}

		if statusCode == 0 {
			statusCode = http.StatusOK
		}
		w.WriteHeader(statusCode)

		if result != nil {
			json.NewEncoder(w).Encode(result) //nolint
		}

	}
}

// WithAuth verify alloation and signature
func WithAuth(r *http.Request) (*Context, error) {

	ctx := &Context{
		Context: context.TODO(),
		Request: r,
		Store:   datastore.GetStore(),
	}

	ctx.Vars = mux.Vars(r)
	if ctx.Vars == nil {
		ctx.Vars = make(map[string]string)
	}

	ctx.ClientID = r.Header.Get(common.ClientHeader)
	ctx.ClientKey = r.Header.Get(common.ClientKeyHeader)
	ctx.AllocationTx = ctx.Vars["allocation"]
	ctx.Signature = r.Header.Get(common.ClientSignatureHeader)

	if len(ctx.AllocationTx) > 0 {
		alloc, err := allocation.GetOrCreate(ctx, ctx.Store, ctx.AllocationTx)

		if err != nil {
			if errors.Is(common.ErrBadRequest, err) {
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
			return ctx, errors.Throw(common.ErrBadRequest, "invalid signature "+ctx.Signature)
		}

		if err != nil {
			ctx.StatusCode = http.StatusInternalServerError
			return ctx, errors.ThrowLog(err.Error(), common.ErrInternal, "invalid signature "+ctx.Signature)
		}
	}

	return ctx, nil
}
