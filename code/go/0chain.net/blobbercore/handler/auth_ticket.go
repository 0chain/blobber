package handler

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/0chain/blobber/code/go/0chain.net/core/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	node2 "github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/0chain/gosdk/zmagmacore/node"
	"go.uber.org/zap"
	"net/http"
)

func GenerateAuthTicket(ctx context.Context, r *http.Request) (interface{}, error) {

	logging.Logger.Info("GenerateAuthTicket")
	selfNode := node2.GetSelfNode(ctx)
	logging.Logger.Info("1GenerateAuthTicket", zap.Any("public_key", selfNode))
	logging.Logger.Info("2GenerateAuthTicket", zap.Any("public_key", selfNode.GetWallet()))
	logging.Logger.Info("3GenerateAuthTicket", zap.Any("client_id", r.URL.Query()))

	clientID := r.URL.Query().Get("client_id")
	ticket := fmt.Sprintf("%s:%s", node.ID(), clientID)

	signatureScheme := zcncrypto.NewSignatureScheme(config.Configuration.SignatureScheme)
	_ = signatureScheme.SetPrivateKey(node.PrivateKey())
	_ = signatureScheme.SetPublicKey(node.PublicKey())

	res, err := signatureScheme.Sign(hex.EncodeToString([]byte(ticket)))
	return res, err
}
