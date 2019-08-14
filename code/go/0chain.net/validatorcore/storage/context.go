package storage

import (
	"context"
	"net/http"

	"0chain.net/core/common"
)

func SetupContext(handler common.JSONResponderF) common.JSONResponderF {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		ctx = context.WithValue(ctx, CLIENT_CONTEXT_KEY, r.Header.Get(common.ClientHeader))
		ctx = context.WithValue(ctx, CLIENT_KEY_CONTEXT_KEY, r.Header.Get(common.ClientKeyHeader))
		res, err := handler(ctx, r)
		return res, err
	}
}
