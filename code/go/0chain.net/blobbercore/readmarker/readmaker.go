package readmarker

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
)

type ReadMakerI interface {
	VerifyMarker(ctx context.Context, sa *allocation.Allocation) error
	GetLatestRM() *ReadMarker
	PendNumBlocks() (pendNumBlocks int64, err error)
}

func NewReadMakerEntity(rm *ReadMarker) ReadMakerI {
	return &ReadMarkerEntity{
		LatestRM: rm,
	}
}
