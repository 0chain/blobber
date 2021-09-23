package chain

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/config"
	"github.com/spf13/viper"
)

/*ServerChain - the chain object of the chain  the server is responsible for */
var ServerChain *Chain

/*SetServerChain - set the server chain object */
func SetServerChain(c *Chain) {
	ServerChain = c
}

/*GetServerChain - returns the chain object for the server chain */
func GetServerChain() *Chain {
	return ServerChain
}

/*Chain - data structure that holds the chain data*/
type Chain struct {
	ID            string
	Version       string
	CreationDate  common.Timestamp
	OwnerID       string
	ParentChainID string
	BlockWorker   string

	GenesisBlockHash string
}

/*Validate - implementing the interface */
func (c *Chain) Validate(ctx context.Context) error {
	if common.IsEmpty(c.ID) {
		return common.InvalidRequest("chain id is required")
	}
	if common.IsEmpty(c.OwnerID) {
		return common.InvalidRequest("owner id is required")
	}
	return nil
}

//NewChainFromConfig - create a new chain from config
func NewChainFromConfig() *Chain {
	chain := Provider()
	chain.ID = common.ToKey(config.Configuration.ChainID)
	chain.OwnerID = viper.GetString("server_chain.owner")
	chain.BlockWorker = viper.GetString("block_worker")
	return chain
}

/*Provider - entity provider for chain object */
func Provider() *Chain {
	c := &Chain{}
	c.Version = "1.0"
	c.InitializeCreationDate()
	return c
}

/*InitializeCreationDate - intializes the creation date for the chain */
func (c *Chain) InitializeCreationDate() {
	c.CreationDate = common.Now()
}
