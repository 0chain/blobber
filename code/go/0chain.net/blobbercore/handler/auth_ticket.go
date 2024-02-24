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
	logging.Logger.Info("1GenerateAuthTicket", zap.Any("client_id", r.URL.Query()), zap.Any("node_id", node.ID()), zap.Any("public_key", node.PublicKey()))
	logging.Logger.Info("2GenerateAuthTicket", zap.Any("private_key", node.PrivateKey()), zap.Any("signature_scheme", config.Configuration.SignatureScheme))

	clientID := r.URL.Query().Get("client_id")
	ticket := fmt.Sprintf("%s:%s", node.ID(), clientID)

	signatureScheme := zcncrypto.NewSignatureScheme(config.Configuration.SignatureScheme)
	_ = signatureScheme.SetPrivateKey(node.PrivateKey())
	_ = signatureScheme.SetPublicKey(node.PublicKey())

	res, err := signatureScheme.Sign(hex.EncodeToString([]byte(ticket)))
	return res, err
}
