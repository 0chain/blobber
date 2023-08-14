//go:build !integration_tests
// +build !integration_tests

package writemarker

import "context"

func (wme *WriteMarkerEntity) RedeemMarker(ctx context.Context) error {
	return wme.redeemMarker(ctx)
}
