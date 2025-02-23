package readmarker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"gorm.io/gorm"
)

type ReadRedeem struct {
	ReadMarker *ReadMarker `json:"read_marker"`
}

func (r *ReadMarkerEntity) BeforeCreate(tx *gorm.DB) error {
	r.CreatedAt = time.Now()
	r.UpdatedAt = r.CreatedAt
	return nil
}

func (r *ReadMarkerEntity) BeforeSave(tx *gorm.DB) error {
	r.UpdatedAt = time.Now()
	return nil
}

// PendNumBlocks Return number of blocks pending to be redeemed
func (rme *ReadMarkerEntity) PendNumBlocks() (pendNumBlocks int64, err error) {
	if rme.LatestRM == nil {
		return 0, common.NewErrorf("rme_pend_num_blocks", "missing latest read marker (nil)")
	}

	pendNumBlocks = rme.LatestRM.ReadCounter - rme.LatestRedeemedRC
	return
}

// RedeemReadMarker redeems the read marker.
func (rme *ReadMarkerEntity) RedeemReadMarker(ctx context.Context) (err error) {
	// Depreciated
	return
}

func GetLatestReadMarkerEntityFromChain(clientID, allocID string) (*ReadMarker, error) {
	params := map[string]string{
		"blobber":    node.Self.ID,
		"client":     clientID,
		"allocation": allocID,
	}

	latestRMBytes, err := transaction.MakeSCRestAPICall(
		transaction.STORAGE_CONTRACT_ADDRESS, "/latestreadmarker", params)

	if err != nil {
		return nil, err
	}
	latestRM := &ReadMarker{}
	err = json.Unmarshal(latestRMBytes, latestRM)
	if err != nil {
		return nil, err
	}
	if latestRM.ClientID == "" { // RMs are not yet redeemed and thus it is empty
		return nil, nil
	}
	return latestRM, nil
}
