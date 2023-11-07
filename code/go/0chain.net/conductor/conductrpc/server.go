package conductrpc

import (
	"errors"

	"github.com/0chain/blobber/code/go/0chain.net/conductor/config"
)

var ErrShutdown = errors.New("server shutdown")

// type aliases
type (
	NodeID    = config.NodeID
	NodeName  = config.NodeName
	Round     = config.Round
	RoundName = config.RoundName
	Number    = config.Number
)

type ValidtorTicket struct {
	ValidatorId string
}
