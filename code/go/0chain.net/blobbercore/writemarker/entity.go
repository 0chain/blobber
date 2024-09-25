package writemarker

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/minio/sha256-simd"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type WriteMarker struct {
	Version                string `gorm:"-" json:"version"`
	AllocationRoot         string `gorm:"column:allocation_root;size:64;primaryKey" json:"allocation_root"`
	PreviousAllocationRoot string `gorm:"column:prev_allocation_root;size:64" json:"prev_allocation_root"`
	FileMetaRoot           string `gorm:"column:file_meta_root;size:64" json:"file_meta_root"`
	AllocationID           string `gorm:"column:allocation_id;size:64;index:idx_seq,unique,priority:1" json:"allocation_id"`
	Size                   int64  `gorm:"column:size" json:"size"`
	ChainSize              int64  `gorm:"column:chain_size" json:"chain_size"`
	// ChainHash is the sha256 hash of the previous chain hash and the current allocation root
	ChainHash   string           `gorm:"column:chain_hash;size:64" json:"chain_hash"`
	ChainLength int              `gorm:"column:chain_length" json:"chain_length"`
	BlobberID   string           `gorm:"column:blobber_id;size:64" json:"blobber_id"`
	Timestamp   common.Timestamp `gorm:"column:timestamp;primaryKey" json:"timestamp"`
	ClientID    string           `gorm:"column:client_id;size:64" json:"client_id"`
	Signature   string           `gorm:"column:signature;size:64" json:"signature"`
}

func (wm *WriteMarker) GetHashData() string {
	hashData := fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s:%d:%d:%d",
		wm.AllocationRoot, wm.PreviousAllocationRoot,
		wm.FileMetaRoot, wm.ChainHash, wm.AllocationID, wm.BlobberID,
		wm.ClientID, wm.Size, wm.ChainSize, wm.Timestamp)
	return hashData
}

type WriteMarkerStatus int

const (
	Accepted   WriteMarkerStatus = 0
	Committed  WriteMarkerStatus = 1
	Failed     WriteMarkerStatus = 2
	Rollbacked WriteMarkerStatus = 3
)

const MARKER_VERSION = "v2"

type WriteMarkerEntity struct {
	// WM new WriteMarker from client
	WM              WriteMarker       `gorm:"embedded"`
	Status          WriteMarkerStatus `gorm:"column:status;not null;default:0"`
	StatusMessage   string            `gorm:"column:status_message"`
	ReedeemRetries  int64             `gorm:"column:redeem_retries;not null;default:0"`
	CloseTxnID      string            `gorm:"column:close_txn_id;size:64"`
	CloseTxnNonce   int64             `gorm:"column:close_txn_nonce"`
	ConnectionID    string            `gorm:"column:connection_id;size:64"`
	ClientPublicKey string            `gorm:"column:client_key;size:256"`
	Latest          bool              `gorm:"column:latest;not null;default:true"`
	Sequence        int64             `gorm:"column:sequence;unique;autoIncrement;<-:false;index:idx_seq,unique,priority:2"` // <-:false skips value insert/update by gorm
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

func (wm *WriteMarkerEntity) UpdateStatus(ctx context.Context, status WriteMarkerStatus, statusMessage, redeemTxn string, startSeq, endSeq int64) (err error) {
	err = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		db := datastore.GetStore().GetTransaction(ctx)
		statusBytes, _ := json.Marshal(statusMessage)

		if status == Failed {
			wm.ReedeemRetries++
			err = db.Model(wm).Updates(WriteMarkerEntity{
				Status:         status,
				StatusMessage:  string(statusBytes),
				CloseTxnID:     redeemTxn,
				ReedeemRetries: wm.ReedeemRetries,
				CloseTxnNonce:  wm.CloseTxnNonce,
			}).Error
			return err
		}

		err = db.Model(wm).Updates(WriteMarkerEntity{
			Status:        status,
			StatusMessage: string(statusBytes),
			CloseTxnID:    redeemTxn,
			CloseTxnNonce: wm.CloseTxnNonce,
		}).Error
		if err != nil {
			return err
		}

		if status == Committed {
			err = db.Exec("UPDATE write_markers SET status=1 WHERE sequence BETWEEN ? AND ? AND allocation_id = ?", startSeq, endSeq, wm.WM.AllocationID).Error
			if err != nil {
				return err
			}
		}

		// TODO (sfxdx): what about failed write markers ?
		if status != Committed || wm.WM.Size <= 0 {
			return err // not committed or a deleting marker
		}

		// work on pre-redeemed tokens and write-pools balances tracking
		if err := allocation.AddToPending(ctx, wm.WM.ClientID, wm.WM.AllocationID, -wm.WM.Size); err != nil {
			return fmt.Errorf("can't save allocation pending value: %v", err)
		}
		return nil
	})

	return
}

func (wm *WriteMarkerEntity) OnChain() bool {
	return wm.Status == Committed
}

// GetWriteMarkerEntity get WriteMarkerEntity from postgres
func GetWriteMarkerEntity(ctx context.Context, allocationID, allocationRoot string) (*WriteMarkerEntity, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	wm := &WriteMarkerEntity{}
	err := db.Table((WriteMarkerEntity{}).TableName()).
		Where("allocation_root=? AND allocation_id=?", allocationRoot, allocationID).
		Order("sequence desc").
		Take(wm).Error
	if err != nil {
		return nil, err
	}
	if wm.Status == Committed {
		wm.WM.ChainLength = 0
	}
	if wm.WM.ChainHash != "" {
		wm.WM.Version = MARKER_VERSION
	}
	return wm, nil
}

// GetPreviousWM get previous WriteMarkerEntity from postgres for rollback WM
func GetPreviousWM(ctx context.Context, allocation_root string, timestamp common.Timestamp) (*WriteMarkerEntity, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	wm := &WriteMarkerEntity{}
	err := db.Table((WriteMarkerEntity{}).TableName()).
		Where("allocation_root <> ? AND prev_allocation_root=? AND timestamp=?", allocation_root, allocation_root, timestamp).
		Order("timestamp desc").
		Take(wm).Error
	if err != nil {
		return nil, err
	}
	return wm, nil
}

// AllocationRootMustUnique allocation_root must be unique in write_markers
func AllocationRootMustUnique(ctx context.Context, allocation_root string, timestamp int64) error {
	db := datastore.GetStore().GetTransaction(ctx)
	var c int64
	db.Raw("SELECT 1 FROM write_markers WHERE allocation_root = ? and timestamp=? and status<>2 ", allocation_root, timestamp).
		Count(&c)

	if c > 0 {
		return common.NewError("write_marker_validation_failed", "Duplicate write marker. Validation failed")
	}

	return nil
}

// TODO: Remove allocation ID after duplicate writemarker fix
func GetWriteMarkersInRange(ctx context.Context, allocationID string, startAllocationRoot string, startTimestamp common.Timestamp, endAllocationRoot string) ([]*WriteMarkerEntity, error) {
	db := datastore.GetStore().GetTransaction(ctx)

	// seq of start allocation root
	startWM := WriteMarkerEntity{}
	err := db.Table((WriteMarkerEntity{}).TableName()).
		Where("allocation_root=? AND timestamp=? AND allocation_id=?", startAllocationRoot, startTimestamp).
		Select("sequence").
		Take(&startWM).Error

	if err != nil {
		logging.Logger.Error("write_marker_not_found", zap.Error(err), zap.Any("allocation_root", startAllocationRoot), zap.Any("timestamp", startTimestamp))
		return nil, common.NewError("write_marker_not_found", "Could not find the start write marker in the range")
	}

	// seq of end allocation root
	endWM := WriteMarkerEntity{}
	err = db.Table((WriteMarkerEntity{}).TableName()).
		Where("allocation_root=? AND allocation_id=?", endAllocationRoot, allocationID).
		Select("sequence").
		Order("sequence desc").
		Take(&endWM).Error
	if err != nil {
		return nil, common.NewError("write_marker_not_found", "Could not find the end write marker in the range")
	}

	retMarkers := make([]*WriteMarkerEntity, 0)
	err = db.Where("allocation_id=? AND sequence BETWEEN ? AND ?",
		allocationID, startWM.Sequence, endWM.Sequence).Order("sequence").
		Find(&retMarkers).Error
	if err != nil {
		return nil, err
	}

	if len(retMarkers) == 0 {
		return nil, common.NewError("write_marker_not_found", "Could not find the write markers in the range")
	}

	if retMarkers[0].WM.AllocationRoot != startAllocationRoot {
		logging.Logger.Error("write_marker_root_mismatch", zap.Any("expected", startAllocationRoot), zap.Any("actual", retMarkers[0].WM.AllocationRoot))
	}

	return retMarkers, nil
}

func (wm *WriteMarkerEntity) Create(ctx context.Context) error {
	db := datastore.GetStore().GetTransaction(ctx)
	err := db.Create(wm).Error
	return err
}

func GetUncommittedWriteMarkers(ctx context.Context, allocationID string, seq int64) ([]*WriteMarkerEntity, error) {
	db := datastore.GetStore().GetTransaction(ctx)

	unCommittedMarkers := make([]*WriteMarkerEntity, 0)
	err := db.Table((WriteMarkerEntity{}).TableName()).
		Where("allocation_id=? AND status=0 AND sequence > ?", allocationID, seq).
		Order("sequence asc").
		Find(&unCommittedMarkers).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return unCommittedMarkers, nil
}

func GetLatestCommittedWriteMarker(ctx context.Context, allocationID string) (*WriteMarkerEntity, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	wm := &WriteMarkerEntity{}
	err := db.Table((WriteMarkerEntity{}).TableName()).
		Where("allocation_id=? AND status=1", allocationID).
		Order("sequence desc").
		Take(wm).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return wm, nil
}

func GetMarkersForChain(ctx context.Context, allocationID string, startSeq, endSeq int64) ([]byte, error) {
	db := datastore.GetStore().GetTransaction(ctx)

	unCommittedMarkers := make([]*WriteMarkerEntity, 0)
	err := db.Table((WriteMarkerEntity{}).TableName()).
		Where("allocation_id=? AND status=0 AND sequence BETWEEN ? AND ?", allocationID, startSeq, endSeq).
		Order("sequence asc").
		Find(&unCommittedMarkers).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	markers := make([]byte, 0, len(unCommittedMarkers))
	for _, marker := range unCommittedMarkers {
		decodedHash, err := hex.DecodeString(marker.WM.AllocationRoot)
		if err != nil {
			return nil, err
		}
		markers = append(markers, decodedHash...)
	}
	return markers, nil
}

func CalculateChainHash(prevChainHash, newRoot string) string {
	hasher := sha256.New()
	if prevChainHash != "" {
		prevBytes, _ := hex.DecodeString(prevChainHash)
		hasher.Write(prevBytes) //nolint:errcheck
	}
	newBytes, _ := hex.DecodeString(newRoot)
	hasher.Write(newBytes) //nolint:errcheck
	return hex.EncodeToString(hasher.Sum(nil))
}
