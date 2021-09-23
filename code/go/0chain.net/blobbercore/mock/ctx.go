package mock

import (
	"context"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/gosdk/constants"
)

func SetupHandlerContext(ctx context.Context, r *http.Request, allocation string) context.Context {

	ctx = context.WithValue(ctx, constants.ContextKeyClient,
		r.Header.Get(common.ClientHeader))
	ctx = context.WithValue(ctx, constants.ContextKeyClientKey,
		r.Header.Get(common.ClientKeyHeader))
	ctx = context.WithValue(ctx, constants.ContextKeyAllocation,
		allocation)
	// signature is not required for all requests, but if header is empty it won`t affect anything
	ctx = context.WithValue(ctx, constants.ContextKeyClientSignatureHeaderKey, r.Header.Get(common.ClientSignatureHeader))
	return ctx
}
