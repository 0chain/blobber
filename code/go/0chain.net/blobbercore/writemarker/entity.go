package writemarker

import (
	"context"
	"fmt"

	"0chain.net/blobbercore/datastore"
	"0chain.net/core/common"
)

type WriteMarker struct {
	AllocationRoot         string           `gorm:"column:allocation_root;primary_key" json:"allocation_root"`
	PreviousAllocationRoot string           `gorm:"column:prev_allocation_root" json:"prev_allocation_root"`
	AllocationID           string           `gorm:"column:allocation_id" json:"allocation_id"`
	Size                   int64            `gorm:"column:size" json:"size"`
	BlobberID              string           `gorm:"column:blobber_id" json:"blobber_id"`
	Timestamp              common.Timestamp `gorm:"column:timestamp" json:"timestamp"`
	ClientID               string           `gorm:"column:client_id" json:"client_id"`
	Signature              string           `gorm:"column:signature" json:"signature"`
}

func (wm *WriteMarker) GetHashData() string {
	hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v", wm.AllocationRoot, wm.PreviousAllocationRoot, wm.AllocationID, wm.BlobberID, wm.ClientID, wm.Size, wm.Timestamp)
	return hashData
}

type WriteMarkerStatus int

const (
	Accepted  WriteMarkerStatus = 0
	Committed WriteMarkerStatus = 1
	Failed    WriteMarkerStatus = 2
)

type WriteMarkerEntity struct {
	WM              WriteMarker       `gorm:"embedded"`
	Status          WriteMarkerStatus `gorm:"column:status"`
	StatusMessage   string            `gorm:"column:status_message"`
	ReedeemRetries  int64             `gorm:"column:redeem_retries"`
	CloseTxnID      string            `gorm:"column:close_txn_id"`
	ConnectionID    string            `gorm:"column:connection_id"`
	ClientPublicKey string            `gorm:"column:client_key"`
	datastore.ModelWithTS
}

func (WriteMarkerEntity) TableName() string {
	return "write_markers"
}

func (wm *WriteMarkerEntity) UpdateStatus(ctx context.Context, status WriteMarkerStatus, status_message string, redeemTxn string) error {
	db := datastore.GetStore().GetTransaction(ctx)
	var err error
	if status == Failed {
		wm.ReedeemRetries++
		err = db.Model(wm).Update(WriteMarkerEntity{Status: status, StatusMessage: status_message, CloseTxnID: redeemTxn, ReedeemRetries: wm.ReedeemRetries}).Error
	} else {
		err = db.Model(wm).Update(WriteMarkerEntity{Status: status, StatusMessage: status_message, CloseTxnID: redeemTxn}).Error
	}
	return err
}

func GetWriteMarkerEntity(ctx context.Context, allocation_root string) (*WriteMarkerEntity, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	wm := &WriteMarkerEntity{}
	err := db.First(wm, "allocation_root = ?", allocation_root).Error
	if err != nil {
		return nil, err
	}
	return wm, nil
}

func GetWriteMarkersInRange(ctx context.Context, allocationID string, startAllocationRoot string, endAllocationRoot string) ([]*WriteMarkerEntity, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	var seqRange []int64
	err := db.Debug().Select("sequence").Where(WriteMarker{AllocationRoot: startAllocationRoot, AllocationID: allocationID}).Or(WriteMarker{AllocationRoot: endAllocationRoot, AllocationID: allocationID}).Find(&seqRange).Error
	if err != nil {
		return nil, err
	}
	if len(seqRange) == 2 {
		retMarkers := make([]*WriteMarkerEntity, 0)
		err = db.Debug().Where("sequence BETWEEN ? AND ?", seqRange[0], seqRange[1]).Order("sequence").Find(&retMarkers).Error
		if err != nil {
			return nil, err
		}
		if len(retMarkers) == 0 {
			return nil, common.NewError("write_marker_not_found", "Could not find the write markers in the range")
		}
		return retMarkers, nil
	}
	return nil, common.NewError("write_marker_not_found", "Could not find the right write markers in the range")
}

func (wm *WriteMarkerEntity) Save(ctx context.Context) error {
	db := datastore.GetStore().GetTransaction(ctx)
	err := db.Save(wm).Error
	return err
}
