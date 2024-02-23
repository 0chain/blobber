package handler

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/0chain/blobber/code/go/0chain.net/core/config"
	"github.com/0chain/common/core/logging"
	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/0chain/gosdk/zmagmacore/node"
	"go.uber.org/zap"
	"net/http"
)

func GenerateAuthTicket(ctx context.Context, r *http.Request) (interface{}, error) {
	logging.Logger.Info("Jayash GenerateAuthTicket")
	clientID := r.URL.Query().Get("client_id")
	logging.Logger.Info("Jayash GenerateAuthTicket", zap.Any("client_id", clientID))
	ticket := fmt.Sprintf("%s:%s", node.ID(), clientID)
	logging.Logger.Info("Jayash GenerateAuthTicket", zap.Any("ticket", ticket))

	signatureScheme := zcncrypto.NewSignatureScheme(config.Configuration.SignatureScheme)
	_ = signatureScheme.SetPrivateKey(node.PrivateKey())
	_ = signatureScheme.SetPublicKey(node.PublicKey())

	res, err := signatureScheme.Sign(hex.EncodeToString([]byte(ticket)))
	logging.Logger.Info("Jayash GenerateAuthTicket", zap.Any("res", res), zap.Any("err", err))
	return res, err
}
