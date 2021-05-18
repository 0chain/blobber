// +build !integration_tests

package handler

import (
	"context"
	"net/http"

	// "0chain.net/blobbercore/config"
	// "0chain.net/blobbercore/constants"
	// "0chain.net/blobbercore/datastore"
	// "0chain.net/blobbercore/stats"
	// "0chain.net/core/common"

	// . "0chain.net/core/logging"
	// "go.uber.org/zap"

	// "github.com/gorilla/mux"

	"0chain.net/blobbercore/reference"
)


func GetOrCreateMarketplaceEncryptionKeyPair(ctx context.Context, r *http.Request) (interface{}, error) {
	info, err := reference.GetOrCreateMarketplaceInfo(ctx)

	return info, err
}
