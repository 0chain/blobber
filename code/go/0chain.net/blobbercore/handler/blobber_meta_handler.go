// +build !integration_tests

package handler

import (
	"net/http"

	// "0chain.net/blobbercore/config"
	// "0chain.net/blobbercore/constants"
	// "0chain.net/blobbercore/datastore"
	// "0chain.net/blobbercore/stats"
	// "0chain.net/core/common"

	// . "0chain.net/core/logging"
	// "go.uber.org/zap"

	// "github.com/gorilla/mux"
)


func GetOrCreateMarketplaceEncryptionKeyPair(r *http.Request) (interface{}, error) {
	data := map[string]string{
		"public_key": "abcd",
	}
	return data, nil
}
