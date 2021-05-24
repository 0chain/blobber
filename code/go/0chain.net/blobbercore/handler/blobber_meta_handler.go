// +build !integration_tests

package handler

import (
	"0chain.net/blobbercore/config"
	"context"
	"net/http"

	"0chain.net/blobbercore/reference"
)


func GetOrCreateMarketplaceEncryptionKeyPair(ctx context.Context, r *http.Request) (*reference.MarketplaceInfo, error) {
	if !config.Configuration.PreEncryption.AutoGenerate {
		mnemonic := config.Configuration.PreEncryption.Mnemonic
		return reference.GetMarketplaceInfoFromMnemonic(mnemonic), nil
	}
	info, err := reference.GetOrCreateMarketplaceInfo(ctx)

	return info, err
}
