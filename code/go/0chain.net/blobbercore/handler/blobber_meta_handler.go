// +build !integration_tests

package handler

import (
	"context"
	"net/http"

	"0chain.net/blobbercore/reference"
)


func GetOrCreateMarketplaceEncryptionKeyPair(ctx context.Context, r *http.Request) (*reference.MarketplaceInfo, error) {
	info, err := reference.GetOrCreateMarketplaceInfo(ctx)

	return info, err
}
