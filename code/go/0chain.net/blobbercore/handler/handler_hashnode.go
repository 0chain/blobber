//go:build !integration_tests
// +build !integration_tests

package handler

import (
	"fmt"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
)

// LoadRootHashnode load root node with its descendant nodes
func LoadRootHashnode(ctx *Context) (interface{}, error) {
	fmt.Println("LoadRootHashnode Start !!!")
	defer func() {
		fmt.Println("LoadRootHashnode End !!!")
	}()
	root, err := reference.LoadRootHashnode(ctx, ctx.AllocationTx)
	if err != nil {
		return nil, err
	}
	return root, nil
}
