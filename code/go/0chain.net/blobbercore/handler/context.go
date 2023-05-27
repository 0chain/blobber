package handler

import (
	"context"
	"encoding/json"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
	"strconv"
	"time"

	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/errors"
	"github.com/gorilla/mux"
)

const (
	defaultMaxMemory = 32 << 20 // 32 MB
)

// Context api context
type Context struct {
	context.Context

	// ClientID client wallet id
	ClientID string
	// ClientKey client wallet public key
	ClientKey string
	// AllocationId optional. allocation id in request
	AllocationId string
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
func (c *Context) FormValue(key string) (string, bool) {
	if c == nil {
		return "", false
	}

	if c.Request.Form == nil {
		c.Request.ParseMultipartForm(defaultMaxMemory) //nolint: errcheck
	}

	if vs := c.Request.Form[key]; len(vs) > 0 {
		return vs[0], true
	}
	return "", false
}

// FormInt get int from form data
func (c *Context) FormInt(key string) (int64, bool) {
	if c == nil {
		return 0, false
	}
	value, ok := c.FormValue(key)

	if !ok || len(value) == 0 {
		return 0, false
	}

	i, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}

	return i, true
}

// FormFloat get float from form data
func (c *Context) FormFloat(key string) (float64, bool) {
	if c == nil {
		return 0, false
	}
	value, ok := c.FormValue(key)

	if !ok || len(value) == 0 {
		return 0, false
	}

	i, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}

	return i, true
}

// FormTime get time from form data
func (c *Context) FormTime(key string) (*time.Time, bool) {
	if c == nil {
		return nil, false
	}

	value, ok := c.FormValue(key)

	if !ok || len(value) == 0 {
		return nil, false
	}

	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, false
	}

	t := time.Unix(seconds, 0)
	return &t, true
}

type ErrorResponse struct {
	Error string
}

// WithHandler process handler to respond request
func WithHandler(handler func(ctx *Context) (interface{}, error)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // CORS for all.
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == "OPTIONS" {
			w.Header().Add("Access-Control-Max-Age", "3600")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		common.TryParseForm(r)

		w.Header().Set("Content-Type", "application/json")

		ctx, err := WithVerify(r)
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

// WithVerify verify allocation and signature
func WithVerify(r *http.Request) (*Context, error) {
	ctx := &Context{
		Context: context.TODO(),
		Request: r,
		Store:   datastore.GetStore(),
	}

	ctx.Vars = mux.Vars(r)
	logging.Logger.Info("jayash Vars", zap.Any("Vars", ctx.Vars))
	if ctx.Vars == nil {
		logging.Logger.Info("jayash Vars is nil")
		ctx.Vars = make(map[string]string)
	}

	ctx.ClientID = r.Header.Get(common.ClientHeader)
	ctx.ClientKey = r.Header.Get(common.ClientKeyHeader)
	ctx.AllocationId = r.Header.Get("allocation_id")
	ctx.Signature = r.Header.Get(common.ClientSignatureHeader)

	logging.Logger.Info("jayash allocationID", zap.Any("allocationID", ctx.AllocationId))

	if len(ctx.AllocationId) > 0 {
		logging.Logger.Info("jayash allocationID is not empty")
		alloc, err := allocation.GetOrCreate(ctx, ctx.Store, ctx.AllocationId)

		logging.Logger.Info("jayash alloc", zap.Any("alloc", alloc), zap.Any("err", err))

		if err != nil {
			logging.Logger.Info("jayash get or create err", zap.Any("err", err))
			if errors.Is(common.ErrBadRequest, err) {
				ctx.StatusCode = http.StatusBadRequest

			} else {
				ctx.StatusCode = http.StatusInternalServerError
			}

			return ctx, err
		}

		ctx.Allocation = alloc

		publicKey := alloc.OwnerPublicKey

		valid, err := verifySignatureFromRequest(ctx.AllocationId, ctx.Signature, publicKey)

		if !valid {
			ctx.StatusCode = http.StatusBadRequest
			return ctx, errors.Throw(common.ErrBadRequest, "invalid signature "+ctx.Signature)
		}

		if err != nil {
			ctx.StatusCode = http.StatusInternalServerError
			return ctx, errors.ThrowLog(err.Error(), common.ErrInternal, "invalid signature "+ctx.Signature)
		}
	} else {
		logging.Logger.Info("jayash allocationID is empty")
		ctx.StatusCode = http.StatusBadRequest
		return ctx, errors.Throw(common.ErrBadRequest, "allocation id is empty")
	}

	return ctx, nil
}
