package handler

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/0chain/blobber/code/go/0chain.net/core/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/0chain/gosdk/zmagmacore/node"
	"go.uber.org/zap"
	"net/http"
)

func GenerateAuthTicket(ctx context.Context, r *http.Request) (interface{}, error) {
	logging.Logger.Info("GenerateAuthTicket")
	logging.Logger.Info("0GenerateAuthTicket", zap.Any("node_id", node.PublicKey()))
	logging.Logger.Info("1GenerateAuthTicket", zap.Any("public_key", node.PublicKey()))
	logging.Logger.Info("2GenerateAuthTicket", zap.Any("private_key", node.PrivateKey()))
	logging.Logger.Info("3GenerateAuthTicket", zap.Any("client_id", r.URL.Query()))
	logging.Logger.Info("4GenerateAuthTicket", zap.Any("id", node.ID()))

	clientID := r.URL.Query().Get("client_id")
	ticket := fmt.Sprintf("%s:%s", node.ID(), clientID)

	signatureScheme := zcncrypto.NewSignatureScheme(config.Configuration.SignatureScheme)
	_ = signatureScheme.SetPrivateKey(node.PrivateKey())
	_ = signatureScheme.SetPublicKey(node.PublicKey())

	res, err := signatureScheme.Sign(hex.EncodeToString([]byte(ticket)))
	return res, err
}
