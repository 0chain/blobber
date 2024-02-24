package handler

import (
	"context"
	"fmt"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/common/core/logging"
	"go.uber.org/zap"
	"net/http"
)

func GenerateAuthTicket(ctx context.Context, r *http.Request) (interface{}, error) {
	logging.Logger.Info("GenerateAuthTicket")
	logging.Logger.Info("1GenerateAuthTicket")
	logging.Logger.Info("2GenerateAuthTicket", zap.Any("query", r.URL.Query()))
	clientID := r.URL.Query().Get("client_id")
	logging.Logger.Info("GenerateAuthTicket", zap.Any("client_id", clientID))
	return node.Self.Sign(fmt.Sprintf("%s:%s", node.Self.ID, clientID))
}
