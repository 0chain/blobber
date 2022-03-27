package readmarker

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
)

type ReadMarkerEntity struct {
	ID             int64       `gorm:"column:id;primary_key" json:"id"`
	ReadMarker     *ReadMarker `gorm:"embedded" json:"read_marker,omitempty"`
	RedeemRequired bool        `gorm:"column:redeem_required"`
	IsSuspend      bool        `gorm:"column:is_suspend" json:"is_suspend"` // is_suspend == true is very unlikely
	StatusMessage  string      `gorm:"column:status_message" json:"status_message"`
	datastore.ModelWithTS
}

func (ReadMarkerEntity) TableName() string {
	return "read_markers"
}

// GetClientPendingRMRedeemTokens Get total tokens that has not been redeemed for some client
func GetClientPendingRMRedeemTokens(ctx context.Context, clientID string) (int64, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	var totalPendingReadBlocks int64
	if err := db.Model(&ReadMarkerEntity{}).Select("sum(read_blocks) as tot_read_blocks").
		Where("client_id = ? AND redeem_required=?", clientID, true).
		Scan(&totalPendingReadBlocks).Error; err != nil {
		return 0, err
	}

	return totalPendingReadBlocks, nil
}

// GetClientPendingReadMarkerEntities Get all read markers that are not redeemed for some client in ascending order by created_at
func GetClientPendingReadMarkerEntities(ctx context.Context, clientID string) ([]*ReadMarkerEntity, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	rms := make([]*ReadMarkerEntity, 0)
	if err := db.Model(&ReadMarkerEntity{}).
		Where("client_id = ? AND redeem_required = ?", clientID, true).Order("created_at ASC").Find(&rms).Error; err != nil {
		return nil, err
	}

	return rms, nil
}

//  GetRedeemRequiringRMEntities Get all read markers that are not redeemed in ascending order by created_at
func GetRedeemRequiringRMEntities(ctx context.Context) ([]*ReadMarkerEntity, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	rms := make([]*ReadMarkerEntity, 0)
	if err := db.Model(&ReadMarkerEntity{}).
		Where("redeem_required = ?", true).Order("created_at ASC").Find(&rms).Error; err != nil {
		return nil, err
	}

	return rms, nil
}

// SaveReadMarker save readmarker which will be redeemed later with redeem worker
func SaveReadMarker(ctx context.Context, rm *ReadMarker) error {
	db := datastore.GetStore().GetTransaction(ctx)
	rmEnt := &ReadMarkerEntity{
		ReadMarker:     rm,
		RedeemRequired: true,
	}

	return db.Create(rmEnt).Error
}

func (rm *ReadMarkerEntity) UpdateStatus(ctx context.Context, redeemRequired, isSuspend bool) error {
	db := datastore.GetStore().GetTransaction(ctx)
	return db.Model(&ReadMarkerEntity{}).Where("id=?", rm.ID).Updates(map[string]interface{}{
		"is_suspend":      isSuspend,
		"redeem_required": redeemRequired,
	}).Error
}
