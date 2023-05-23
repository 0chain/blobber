package readmarker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	zLogger "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/gosdk/constants"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Lock write access to readmarkers table for specific client:allocationId combination.
// SaveLatestReadMarker can cause inconsistency without locking mechanism because it is called in two places and can be executed
// independently. Its better to return error if there is already a lock. Client reading file needs to resend read request.
// For readmarker worker it will simply redeem in next interval.
// UpdateStatus also updates readmarker but it does not require lock as it only updates `latest_redeemed_rc` and no other process
// updates it.
// Mostly SaveLatestReadMarker will be called while DownloadFile is called. So it won't be blocking for client.
// Only when blobber is not in sync with blockchain, SaveLatestReadMarker will be called.
var ReadmarkerMapLock = common.GetNewLocker()

type ReadMarker struct {
	ClientID        string           `gorm:"column:client_id;size:64;primaryKey" json:"client_id"`
	AllocationID    string           `gorm:"column:allocation_id;size:64;primaryKey" json:"allocation_id"`
	ClientPublicKey string           `gorm:"column:client_public_key;size:128" json:"client_public_key"`
	BlobberID       string           `gorm:"-" json:"blobber_id"`
	OwnerID         string           `gorm:"column:owner_id;size:64" json:"owner_id"`
	Timestamp       common.Timestamp `gorm:"column:timestamp" json:"timestamp"`
	ReadCounter     int64            `gorm:"column:counter" json:"counter"`
	Signature       string           `gorm:"column:signature;size:64" json:"signature"`
	SessionRC       int64            `gorm:"column:session_rc" json:"session_rc"`
}

func (rm *ReadMarker) GetHashData() string {
	hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v", rm.AllocationID,
		rm.BlobberID, rm.ClientID, rm.ClientPublicKey, rm.OwnerID,
		rm.ReadCounter, rm.Timestamp)
	return hashData
}

type ReadMarkerEntity struct {
	LatestRM         *ReadMarker `gorm:"embedded" json:"latest_read_marker,omitempty"`
	LatestRedeemedRC int64       `gorm:"latest_redeemed_rc" json:"latest_redeemed_rc"`
	datastore.ModelWithTS
}

func (ReadMarkerEntity) TableName() string {
	return "read_markers"
}

func (rm *ReadMarkerEntity) VerifyMarker(ctx context.Context, sa *allocation.Allocation) error {
	if rm == nil || rm.LatestRM == nil {
		return common.NewError("invalid_read_marker", "No read marker was found")
	}
	if rm.LatestRM.AllocationID != sa.ID {
		return common.NewError("read_marker_validation_failed", "Read Marker is not for the same allocation")
	}

	if rm.LatestRM.BlobberID != node.Self.ID {
		return common.NewError("read_marker_validation_failed", "Read Marker is not for the blobber")
	}

	if rm.LatestRM.OwnerID != sa.OwnerID {
		return common.NewError("read_marker_validation_failed", "OwnerID mismatch")
	}

	clientPublicKey := ctx.Value(constants.ContextKeyClientKey).(string)
	if clientPublicKey == "" || clientPublicKey != rm.LatestRM.ClientPublicKey {
		return common.NewError("read_marker_validation_failed", "Could not get the public key of the client")
	}

	clientID := ctx.Value(constants.ContextKeyClient).(string)
	if clientID == "" || clientID != rm.LatestRM.ClientID {
		return common.NewError("read_marker_validation_failed", "Read Marker clientID does not match request clientID")
	}
	currentTS := common.Now()
	if rm.LatestRM.Timestamp > currentTS {
		zLogger.Logger.Error("Timestamp is for future in the read marker", zap.Any("rm", rm), zap.Any("now", currentTS))
		return common.NewError("read_marker_validation_failed", "Timestamp is for future in the read marker")
	}

	hashData := rm.LatestRM.GetHashData()
	signatureHash := encryption.Hash(hashData)
	sigOK, err := encryption.Verify(clientPublicKey, rm.LatestRM.Signature, signatureHash)
	if err != nil {
		return common.NewError("read_marker_validation_failed", "Error during verifying signature. "+err.Error())
	}
	if !sigOK {
		return common.NewError("read_marker_validation_failed", "Read marker signature is not valid")
	}
	return nil
}

func GetLatestReadMarkerEntity(ctx context.Context, clientID, allocID string) (*ReadMarkerEntity, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	rm := &ReadMarkerEntity{}
	err := db.First(rm, "client_id = ? AND allocation_id = ?", clientID, allocID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		latestRM, err := GetLatestReadMarkerEntityFromChain(clientID, allocID)
		if err != nil {
			return nil, err
		}
		if latestRM == nil {
			return nil, nil
		}

		rm.LatestRM = latestRM
		rm.LatestRedeemedRC = latestRM.ReadCounter
		err = db.Create(rm).Error
		if err != nil {
			return nil, err
		}
		return rm, nil
	}

	if err != nil {
		return nil, err
	}
	return rm, nil
}

func GetRedeemRequiringRMEntities(ctx context.Context) ([]*ReadMarkerEntity, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	var rms []*ReadMarkerEntity
	err := db.Model(&ReadMarkerEntity{}).Where("counter > latest_redeemed_rc").Order("updated_at asc").Find(&rms).Error
	if err != nil {
		return nil, err
	}
	return rms, nil
}

// SaveLatestReadMarker will save latest readmarker for a client. Basically it updates `read_counter` of the
// readmarker. There can be multiple requests from same client and `read_counter` update can become inconsis-
// tent. So to make sure we have reliable `read_counter` value, a lock is required to be called before calling
// this function. We should use lock from `ReadMarkerMapLock`
func SaveLatestReadMarker(ctx context.Context, rm *ReadMarker, latestRedeemedRC int64, isCreate bool) error {
	db := datastore.GetStore().GetTransaction(ctx)
	rmEntity := &ReadMarkerEntity{
		LatestRM: rm,
	}

	if latestRedeemedRC != 0 {
		rmEntity.LatestRedeemedRC = latestRedeemedRC
	}

	if isCreate {
		return db.Create(rmEntity).Error
	}
	return db.Model(rmEntity).Updates(map[string]interface{}{
		"timestamp": rm.Timestamp,
		"counter":   rm.ReadCounter,
		"signature": rm.Signature,
	}).Error
}

// Sync read marker with 0chain to be sure its correct.
func (rm *ReadMarkerEntity) Sync(ctx context.Context) error {
	var db = datastore.GetStore().GetTransaction(ctx)
	// update local read pools cache from sharders
	rp, err := allocation.RequestReadPoolStat(rm.LatestRM.ClientID)
	if err != nil {
		return common.NewErrorf("rme_sync", "can't get read pools from sharders: %v", err)
	}

	// save the fresh read pools information
	err = allocation.SetReadPool(db, rp)
	if err != nil {
		return common.NewErrorf("rme_sync", "can't update read pools from sharders: %v", err)
	}

	return err
}

// UpdateStatus updates read marker status and all related on successful redeeming.
func (rme *ReadMarkerEntity) UpdateStatus(ctx context.Context, txOutput, redeemTxn string) (err error) {
	var redeems []allocation.ReadPoolRedeem
	if err = json.Unmarshal([]byte(txOutput), &redeems); err != nil {
		zLogger.Logger.Error("update read redeeming status: can't decode transaction output", zap.Error(err))
		return common.NewErrorf("rme_update_status", "can't decode transaction output: %v", err)
	}

	var db = datastore.GetStore().GetTransaction(ctx)

	err = db.Model(rme).
		Where("client_id=?", rme.LatestRM.ClientID).
		Update("latest_redeemed_rc", rme.LatestRM.ReadCounter).Error
	if err != nil {
		return common.NewError("rme_update_status", err.Error())
	}

	rp, err := allocation.RequestReadPoolStat(rme.LatestRM.ClientID)
	if err != nil {
		return common.NewErrorf("rme_update_status", "can't get read pools from sharders: %v", err)
	}

	if err := allocation.UpdateReadPool(db, rp); err != nil {
		return common.NewErrorf("rme_update_status", "can't update local read pools cache: %v", err)
	}

	return
}
