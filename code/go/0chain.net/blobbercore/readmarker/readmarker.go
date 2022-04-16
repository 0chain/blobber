package readmarker

import (
	"context"
	"fmt"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	zLogger "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/gosdk/constants"
	"go.uber.org/zap"

	"gorm.io/datatypes"
)

type ReadRedeem struct {
	ReadMarker *ReadMarker `json:"read_marker"`
}

type ReadMarker struct {
	ClientID        string           `gorm:"column:client_id;size:64" json:"client_id"`
	ClientPublicKey string           `gorm:"column:client_public_key;size:128" json:"client_public_key"`
	BlobberID       string           `gorm:"-" json:"blobber_id"`
	AllocationID    string           `gorm:"column:allocation_id;size:64" json:"allocation_id"`
	OwnerID         string           `gorm:"column:owner_id;size:64" json:"owner_id"`
	Timestamp       common.Timestamp `gorm:"column:timestamp" json:"timestamp"`

	ReadSize int64 `gorm:"column:read_size" json:"read_size"`

	Signature string `gorm:"column:signature;size:64" json:"signature"`
	PayerID   string `gorm:"column:payer_id;size:64" json:"payer_id"`

	// Remove this as well
	AuthTicket datatypes.JSON `gorm:"column:auth_ticket" json:"auth_ticket"`
}

func (rm *ReadMarker) GetHashData() string {
	hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v", rm.AllocationID,
		rm.BlobberID, rm.ClientID, rm.ClientPublicKey, rm.OwnerID,
		rm.ReadSize, rm.Timestamp)
	return hashData
}

func (rm *ReadMarkerEntity) VerifyMarker(ctx context.Context, sa *allocation.Allocation) error {
	if rm == nil || rm.ReadMarker == nil {
		return common.NewError("invalid_read_marker", "No read marker was found")
	}
	if rm.ReadMarker.AllocationID != sa.ID {
		return common.NewError("read_marker_validation_failed", "Read Marker is not for the same allocation")
	}

	if rm.ReadMarker.BlobberID != node.Self.ID {
		return common.NewError("read_marker_validation_failed", "Read Marker is not for the blobber")
	}

	clientPublicKey := ctx.Value(constants.ContextKeyClientKey).(string)
	if clientPublicKey == "" || clientPublicKey != rm.ReadMarker.ClientPublicKey {
		return common.NewError("read_marker_validation_failed", "Could not get the public key of the client")
	}

	clientID := ctx.Value(constants.ContextKeyClient).(string)
	if clientID == "" || clientID != rm.ReadMarker.ClientID {
		return common.NewError("read_marker_validation_failed", "Read Marker clientID does not match request clientID")
	}
	currentTS := common.Now()
	if rm.ReadMarker.Timestamp > currentTS {
		zLogger.Logger.Error("Timestamp is for future in the read marker", zap.Any("rm", rm), zap.Any("now", currentTS))
	}
	currentTS = common.Now()
	if rm.ReadMarker.Timestamp > (currentTS + 2) {
		zLogger.Logger.Error("Timestamp is for future in the read marker", zap.Any("rm", rm), zap.Any("now", currentTS))
		return common.NewError("read_marker_validation_failed", "Timestamp is for future in the read marker")
	}

	hashData := rm.ReadMarker.GetHashData()
	signatureHash := encryption.Hash(hashData)
	sigOK, err := encryption.Verify(clientPublicKey, rm.ReadMarker.Signature, signatureHash)
	if err != nil {
		return common.NewError("read_marker_validation_failed", "Error during verifying signature. "+err.Error())
	}
	if !sigOK {
		return common.NewError("read_marker_validation_failed", "Read marker signature is not valid")
	}
	return nil
}
