package storage

import (
	"context"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/gosdk/constants"
)

func SetupContext(handler common.JSONResponderF) common.JSONResponderF {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		ctx = context.WithValue(ctx, constants.ContextKeyClient, r.Header.Get(common.ClientHeader))
		ctx = context.WithValue(ctx, constants.ContextKeyClientKey,
			r.Header.Get(common.ClientKeyHeader))
		res, err := handler(ctx, r)
		return res, err
	}
}
