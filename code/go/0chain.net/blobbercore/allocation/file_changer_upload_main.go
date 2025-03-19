//go:build !integration_tests
// +build !integration_tests

package allocation

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

func (nf *UploadFileChanger) ApplyChange(ctx context.Context,
	ts common.Timestamp, allocationVersion int64, collector reference.QueryCollector) error {
	return nf.applyChange(ctx, ts, allocationVersion, collector)
}
