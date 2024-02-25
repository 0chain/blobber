package handler

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/common/core/common"
	"go.uber.org/zap"
	"net/http"
)

func GenerateAuthTicket(ctx context.Context, r *http.Request) (interface{}, error) {
	logging.Logger.Info("GenerateAuthTicket", zap.Any("query", r.URL.Query()))
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		return nil, common.NewError("missing_client_id", "client_id is required")
	}
	logging.Logger.Info("GenerateAuthTicket", zap.Any("client_id", clientID))
	return node.Self.Sign(clientID)
}
