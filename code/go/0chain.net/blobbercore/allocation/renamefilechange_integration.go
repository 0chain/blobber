//go:build integration_tests
// +build integration_tests

package allocation

import (
	"context"
	"errors"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/conductor/conductrpc"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
)

func (rf *RenameFileChange) ApplyChange(ctx context.Context, rootRef *reference.Ref, change *AllocationChange,
	allocationRoot string, ts common.Timestamp, _ map[string]string) (*reference.Ref, error) {

	state := conductrpc.Client().State()
	if state.FailRenameCommit != nil {
		for _, nodeId := range state.FailRenameCommit {
			if nodeId == node.Self.ID {
				return nil, errors.New("error directed by conductor")
			}
		}
	}
	return rf.applyChange(ctx, rootRef, change, allocationRoot, ts, nil)
}
