package conductrpc

import (
	"net/rpc"
)

// client of the conductor RPC server.
type client struct {
	address string      // RPC server address
	client  *rpc.Client // RPC client
}

// newClient creates new client will be interacting
// with server with given address.
func newClient(address string) (c *client, err error) {
	if address, err = Host(address); err != nil {
		return
	}
	c = new(client)
	if c.client, err = rpc.Dial("tcp", address); err != nil {
		return nil, err
	}
	c.address = address
	return
}

func (c *client) dial() (err error) { //nolint:unused,deadcode // might be used later?
	c.client, err = rpc.Dial("tcp", c.address)
	return
}

// Address of RPC server.
func (c *client) Address() string {
	return c.address
}

//
// miner SC RPC
//

// state requests current client state using long polling strategy. E.g.
// when the state had updated, then the method returns.
func (c *client) state(me NodeID) (state *State, err error) {
	err = c.client.Call("Server.State", me, &state)
	for err == rpc.ErrShutdown {
		err = c.client.Call("Server.State", me, &state)
	}
	return
}

func (c *client) blobberCommitted(blobberID string) (err error) {
	err = c.client.Call("Server.BlobberCommitted", blobberID, nil)
	return
}

func (c *client) validationTicketGenerated(ticket ValidtorTicket) (err error) {
	err = c.client.Call("Server.ValidatorTicket", &ticket, nil)
	return
}