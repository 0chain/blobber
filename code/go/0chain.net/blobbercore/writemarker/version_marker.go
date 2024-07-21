package writemarker

import (
	"context"
	"fmt"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

type VersionMarker struct {
	ID            int64  `gorm:"column:sequence;primaryKey"`
	AllocationID  string `gorm:"allocation_id" json:"allocation_id"`
	Version       int64  `gorm:"version" json:"version"`
	Timestamp     int64  `gorm:"timestamp" json:"timestamp"`
	Signature     string `gorm:"signature" json:"signature"`
	IsRepair      bool   `gorm:"is_repair" json:"is_repair"`
	RepairVersion int64  `gorm:"repair_version" json:"repair_version"`
	RepairOffset  string `gorm:"repair_offset" json:"repair_offset"`
}

func (VersionMarker) TableName() string {
	return "version_markers"
}

func GetCurrentVersion(ctx context.Context, allocationID string) (*VersionMarker, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	var vm VersionMarker
	err := db.Where("allocation_id = ?", allocationID).Order("id DESC").Take(&vm).Error
	return &vm, err
}

func (vm *VersionMarker) Verify(allocationID, clientPubKey string) error {
	if vm.AllocationID != allocationID {
		return common.NewError("version_marker_validation_failed", "Invalid allocation id")
	}

	if vm.Signature == "" {
		return common.NewError("version_marker_validation_failed", "Signature is missing")
	}

	hashData := vm.GetHashData()
	signatureHash := encryption.Hash(hashData)
	sigOK, err := encryption.Verify(clientPubKey, vm.Signature, signatureHash)
	if err != nil {
		return common.NewError("version_marker_validation_failed", "Error during verifying signature. "+err.Error())
	}
	if !sigOK {
		logging.Logger.Error("write_marker_sig_error", zap.Any("vm", vm))
		return common.NewError("version_marker_validation_failed", "Version marker signature is not valid")
	}
	return nil
}

func (vm *VersionMarker) GetHashData() string {
	return fmt.Sprintf("%s:%d:%d", vm.AllocationID, vm.Version, vm.Timestamp)
}
