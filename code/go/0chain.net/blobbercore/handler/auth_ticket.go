package handler

import (
	"context"
	"fmt"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"net/http"
)

func GenerateAuthTicket(ctx context.Context, r *http.Request) (interface{}, error) {
	clientID := r.URL.Query().Get("client_id")
	return node.Self.Sign(fmt.Sprintf("%s:%s", node.Self.ID, clientID))
}
