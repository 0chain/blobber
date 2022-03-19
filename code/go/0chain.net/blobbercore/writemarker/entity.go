package writemarker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"gorm.io/gorm"
)

type WriteMarker struct {
	AllocationRoot         string           `gorm:"column:allocation_root;size:64;primary_key" json:"allocation_root"`
	PreviousAllocationRoot string           `gorm:"column:prev_allocation_root;size:64;not null" json:"prev_allocation_root"`
	AllocationID           string           `gorm:"column:allocation_id;size:64;not null" json:"allocation_id"`
	Size                   int64            `gorm:"column:size;not null;default:0" json:"size"`
	BlobberID              string           `gorm:"column:blobber_id;size:64;not null" json:"blobber_id"`
	Timestamp              common.Timestamp `gorm:"column:timestamp;not null" json:"timestamp"`
	ClientID               string           `gorm:"column:client_id;size:64;not null" json:"client_id"`
	Signature              string           `gorm:"column:signature;size:256;not null" json:"signature"`
}

func (wm *WriteMarker) GetHashData() string {
	hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v", wm.AllocationRoot,
		wm.PreviousAllocationRoot, wm.AllocationID, wm.BlobberID, wm.ClientID,
		wm.Size, wm.Timestamp)
	return hashData
}

type WriteMarkerStatus int

const (
	Accepted  WriteMarkerStatus = 0
	Committed WriteMarkerStatus = 1
	Failed    WriteMarkerStatus = 2
)

type WriteMarkerEntity struct {
	// WM new WriteMarker from client
	WM              WriteMarker       `gorm:"embedded"`
	Status          WriteMarkerStatus `gorm:"column:status;not null;default:0"`
	StatusMessage   string            `gorm:"column:status_message"`
	ReedeemRetries  int64             `gorm:"column:redeem_retries;not null;default:0"`
	CloseTxnID      string            `gorm:"column:close_txn_id;size:64"`
	ConnectionID    string            `gorm:"column:connection_id;size:64"`
	ClientPublicKey string            `gorm:"column:client_key;size:256"`
	datastore.ModelWithTS
}

func (WriteMarkerEntity) TableName() string {
	return "write_markers"
}

func (w *WriteMarkerEntity) BeforeCreate(tx *gorm.DB) error {
	w.CreatedAt = time.Now()
	w.UpdatedAt = w.CreatedAt
	return nil
}

func (w *WriteMarkerEntity) BeforeSave(tx *gorm.DB) error {
	w.UpdatedAt = time.Now()
	return nil
}

func (wm *WriteMarkerEntity) UpdateStatus(ctx context.Context, status WriteMarkerStatus, statusMessage, redeemTxn string) (err error) {
	db := datastore.GetStore().GetTransaction(ctx)
	statusBytes, _ := json.Marshal(statusMessage)

	if status == Failed {
		wm.ReedeemRetries++
		err = db.Model(wm).Updates(WriteMarkerEntity{
			Status:         status,
			StatusMessage:  string(statusBytes),
			CloseTxnID:     redeemTxn,
			ReedeemRetries: wm.ReedeemRetries,
		}).Error
		return
	}

	err = db.Model(wm).Updates(WriteMarkerEntity{
		Status:        status,
		StatusMessage: string(statusBytes),
		CloseTxnID:    redeemTxn,
	}).Error
	if err != nil {
		return
	}

	// TODO (sfxdx): what about failed write markers ?
	if status != Committed || wm.WM.Size <= 0 {
		return // not committed or a deleting marker
	}

	// work on pre-redeemed tokens and write-pools balances tracking

	var pend *allocation.Pending
	pend, err = allocation.GetPending(db, wm.WM.ClientID, wm.WM.AllocationID,
		wm.WM.BlobberID)
	if err != nil {
		return fmt.Errorf("can't get allocation pending values: %v", err)
	}
	pend.SubPendingWrite(wm.WM.Size)
	if err = pend.Save(db); err != nil {
		return fmt.Errorf("can't save allocation pending value: %v", err)
	}
	return
}

// GetWriteMarkerEntity get WriteMarkerEntity from postgres
func GetWriteMarkerEntity(ctx context.Context, allocation_root string) (*WriteMarkerEntity, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	wm := &WriteMarkerEntity{}
	err := db.First(wm, "allocation_root = ?", allocation_root).Error
	if err != nil {
		return nil, err
	}
	return wm, nil
}

// AllocationRootMustUnique allocation_root must be unique in write_markers
func AllocationRootMustUnique(ctx context.Context, allocation_root string) error {
	db := datastore.GetStore().GetTransaction(ctx)

	var c int64
	db.Raw("SELECT 1 FROM write_markers WHERE allocation_root = ? and status<>2 ", allocation_root).
		Count(&c)

	if c > 0 {
		return common.NewError("write_marker_validation_failed", "Duplicate write marker. Validation failed")
	}

	return nil
}

func GetWriteMarkersInRange(ctx context.Context, allocationID, startAllocationRoot, endAllocationRoot string) ([]*WriteMarkerEntity, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	var seqRange []int64
	err := db.Table((WriteMarkerEntity{}).TableName()).
		Where(WriteMarker{AllocationRoot: startAllocationRoot, AllocationID: allocationID}).
		Or(WriteMarker{AllocationRoot: endAllocationRoot, AllocationID: allocationID}).
		Order("sequence").
		Pluck("sequence", &seqRange).Error
	if err != nil {
		return nil, err
	}
	if len(seqRange) == 1 {
		seqRange = append(seqRange, seqRange[0])
	}
	if len(seqRange) == 2 {
		retMarkers := make([]*WriteMarkerEntity, 0)
		err = db.Where("sequence BETWEEN ? AND ?", seqRange[0], seqRange[1]).Order("sequence").Find(&retMarkers).Error
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
