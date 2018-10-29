package chain

import (
	"context"

	"0chain.net/common"
	"0chain.net/config"
	"0chain.net/node"
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

	/*Miners - this is the pool of miners */
	Miners *node.Pool

	/*Sharders - this is the pool of sharders */
	Sharders *node.Pool

	/*Blobbers - this is the pool of blobbers */
	Blobbers *node.Pool

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
	return chain
}

/*Provider - entity provider for chain object */
func Provider() *Chain {
	c := &Chain{}
	c.Initialize()
	c.Version = "1.0"
	c.InitializeCreationDate()
	c.Miners = node.NewPool(node.NodeTypeMiner)
	c.Sharders = node.NewPool(node.NodeTypeSharder)
	c.Blobbers = node.NewPool(node.NodeTypeBlobber)
	return c
}

/*Initialize - intializes internal datastructures to start again */
func (c *Chain) Initialize() {

}

/*InitializeCreationDate - intializes the creation date for the chain */
func (c *Chain) InitializeCreationDate() {
	c.CreationDate = common.Now()
}
