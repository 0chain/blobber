package handler

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/common/core/logging"
	"go.uber.org/zap"
	"net/http"
)

func GenerateAuthTicket(ctx context.Context, r *http.Request) (interface{}, error) {
	result, err := common.GenerateAuthTicket(r.URL.Query().Get("client_id"))
	logging.Logger.Info("GenerateAuthTicket", zap.Any("result", result), zap.Any("err", err))
	return result, err
}
