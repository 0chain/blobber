//go:build integration_tests
// +build integration_tests

package writemarker

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/conductor/conductrpc"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
)

func (wme *WriteMarkerEntity) RedeemMarker(ctx context.Context) error {
	err := wme.redeemMarker(ctx)
	if err == nil {
		// send state to conductor server
		conductrpc.Client().BlobberCommitted(node.Self.ID)
	}
	return err
}
