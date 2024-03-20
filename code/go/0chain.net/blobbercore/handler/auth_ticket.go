package handler

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/common/core/common"
	"net/http"
)

func GenerateAuthTicket(ctx context.Context, r *http.Request) (interface{}, error) {
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		return nil, common.NewError("missing_client_id", "client_id is required")
	}

	signature, err := node.Self.Sign(clientID)
	if err != nil {
		return nil, common.NewError("signature_failed", "signature failed")
	}

	return map[string]interface{}{
		"auth_ticket": signature,
	}, nil
}
