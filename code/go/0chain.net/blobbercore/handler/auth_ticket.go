package handler

import (
	"context"
	"fmt"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/common/core/common"
	"github.com/0chain/gosdk/core/encryption"
	"net/http"
)

// swagger:model AuthTicketResponse
type AuthTicketResponse struct {
	AuthTicket string `json:"auth_ticket"`
}

// swagger:route GET /v1/auth/generate GetAuthTicket
// Generate blobber authentication ticket.
//
// Generate and retrieve blobber authentication ticket signed by the blobber's signature. Used by restricted blobbers to enable users to use them to host allocations.
//
// parameters:
//
//	+name: Zbox-Signature
//	 in: header
//	 type: string
//	 description: Digital signature to verify that the sender is 0box service.
//	+name: client_id
//	 type: string
//	 in: query
//	 description: Client ID is used as a payload to the token generated. The token represents a signed version of this string by the blobber's private key.
//
// responses:
//
//	200: AuthTicketResponse
func GenerateAuthTicket(ctx context.Context, r *http.Request) (interface{}, error) {

	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		return nil, common.NewError("missing_client_id", "client_id is required")
	}

	round := r.URL.Query().Get("round")

	payload := encryption.Hash(fmt.Sprintf("%s_%s", clientID, round))

	signature, err := node.Self.Sign(payload)
	if err != nil {
		return nil, common.NewError("signature_failed", "signature failed")
	}
	return &AuthTicketResponse{
		AuthTicket: signature,
	}, nil
}
