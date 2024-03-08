//go:build !integration_tests
// +build !integration_tests

package writemarker

import "context"

func (wme *WriteMarkerEntity) RedeemMarker(ctx context.Context, startSeq int64) error {
	return wme.redeemMarker(ctx, startSeq)
}
