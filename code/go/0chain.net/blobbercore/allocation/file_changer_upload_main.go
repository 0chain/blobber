//go:build !integration_tests
// +build !integration_tests

package allocation

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

func (nf *UploadFileChanger) ApplyChange(ctx context.Context, rootRef *reference.Ref, change *AllocationChange,
	allocationRoot string, ts common.Timestamp, fileIDMeta map[string]string) (*reference.Ref, error) {
	return nf.applyChange(ctx, rootRef, change, allocationRoot, ts, fileIDMeta)
}
