package handler

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/0chain/blobber/code/go/0chain.net/core/config"
	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/0chain/gosdk/zmagmacore/node"
	"net/http"
)

func GenerateAuthTicket(ctx context.Context, r *http.Request) (interface{}, error) {

	clientID := r.URL.Query().Get("client_id")
	ticket := fmt.Sprintf("%s:%s", node.ID(), clientID)

	signatureScheme := zcncrypto.NewSignatureScheme(config.Configuration.SignatureScheme)
	_ = signatureScheme.SetPrivateKey(node.PrivateKey())
	_ = signatureScheme.SetPublicKey(node.PublicKey())

	return signatureScheme.Sign(hex.EncodeToString([]byte(ticket)))
}
