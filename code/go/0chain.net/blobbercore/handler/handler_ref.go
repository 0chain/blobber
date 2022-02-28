//go:build !integration_tests
// +build !integration_tests

package handler

import "github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"

// LoadRootNode load root node with its descendant nodes
func LoadRootNode(ctx *Context) (interface{}, error) {

	root, err := reference.LoadRootNode(ctx, ctx.AllocationTx)
	if err != nil {
		return nil, err
	}
	return root, nil
}
