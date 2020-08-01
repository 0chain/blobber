package conductrpc

import (
	"errors"

	"0chain.net/conductor/config"
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
