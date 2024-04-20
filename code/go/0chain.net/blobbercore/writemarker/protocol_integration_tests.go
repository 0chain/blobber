//go:build integration_tests
// +build integration_tests

package writemarker

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/conductor/conductrpc"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
)

func (wme *WriteMarkerEntity) RedeemMarker(ctx context.Context, startSeq int64) error {
	for {
		state := conductrpc.Client().State()
		if state.StopWMCommit != nil && *state.StopWMCommit {
			time.Sleep(time.Second * 5)
			continue
		}
		break
	}
	err := wme.redeemMarker(ctx, startSeq)
	if err == nil {
		// send state to conductor server
		conductrpc.Client().BlobberCommitted(node.Self.ID)
	}
	return err
}
