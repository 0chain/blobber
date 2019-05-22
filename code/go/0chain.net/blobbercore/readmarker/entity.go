package readmarker

import (
	"context"
	"encoding/json"
	"fmt"

	"0chain.net/blobbercore/allocation"
	"0chain.net/blobbercore/datastore"
	"0chain.net/core/common"
	"0chain.net/core/encryption"

	"github.com/jinzhu/gorm/dialects/postgres"
)

type AuthTicket struct {
	ClientID     string           `json:"client_id"`
	OwnerID      string           `json:"owner_id"`
	AllocationID string           `json:"allocation_id"`
	FilePathHash string           `json:"file_path_hash"`
	FileName     string           `json:"file_name"`
	Expiration   common.Timestamp `json:"expiration"`
	Timestamp    common.Timestamp `json:"timestamp"`
	Signature    string           `json:"signature"`
}

func (rm *AuthTicket) GetHashData() string {
	hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v", rm.AllocationID, rm.ClientID, rm.OwnerID, rm.FilePathHash, rm.FileName, rm.Expiration, rm.Timestamp)
	return hashData
}

func (authToken *AuthTicket) Verify(allocationObj *allocation.Allocation, filename string, pathHash string, clientID string) error {
	if authToken.AllocationID != allocationObj.ID {
		return common.NewError("invalid_parameters", "Invalid auth ticket. Allocation id mismatch")
	}
	if authToken.ClientID != clientID && len(authToken.ClientID) > 0 {
		return common.NewError("invalid_parameters", "Invalid auth ticket. Client ID mismatch")
	}
	if authToken.Expiration < authToken.Timestamp || authToken.Expiration < common.Now() {
		return common.NewError("invalid_parameters", "Invalid auth ticket. Expired ticket")
	}
	if authToken.FilePathHash != pathHash {
		return common.NewError("invalid_parameters", "Invalid auth ticket. Path hash mismatch")
	}
	if authToken.OwnerID != allocationObj.OwnerID {
		return common.NewError("invalid_parameters", "Invalid auth ticket. Owner ID mismatch")
	}
	if authToken.Timestamp > (common.Now() + 2) {
		return common.NewError("invalid_parameters", "Invalid auth ticket. Timestamp in future")
	}
	if authToken.FileName != filename {
		return common.NewError("invalid_parameters", "Invalid auth ticket. Filename mismatch")
	}
	hashData := authToken.GetHashData()
	signatureHash := encryption.Hash(hashData)
	sigOK, err := encryption.Verify(allocationObj.OwnerPublicKey, authToken.Signature, signatureHash)
	if err != nil || !sigOK {
		return common.NewError("invalid_parameters", "Invalid auth ticket. Signature verification failed")
	}
	return nil
}

type ReadMarker struct {
	ClientID        string           `gorm:"column:client_id;primary_key" json:"client_id"`
	ClientPublicKey string           `gorm:"column:client_public_key" json:"client_public_key"`
	BlobberID       string           `gorm:"column:blobber_id" json:"blobber_id"`
	AllocationID    string           `gorm:"column:allocation_id" json:"allocation_id"`
	OwnerID         string           `gorm:"column:owner_id" json:"owner_id"`
	Timestamp       common.Timestamp `gorm:"column:timestamp" json:"timestamp"`
	ReadCounter     int64            `gorm:"column:counter" json:"counter"`
	Signature       string           `gorm:"column:signature" json:"signature"`
}

func (rm *ReadMarker) GetHashData() string {
	hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v", rm.AllocationID, rm.BlobberID, rm.ClientID, rm.ClientPublicKey, rm.OwnerID, rm.ReadCounter, rm.Timestamp)
	return hashData
}

type ReadMarkerEntity struct {
	LatestRM             *ReadMarker    `gorm:"embedded" json:"latest_read_marker,omitempty"`
	LatestRedeemedRMBlob postgres.Jsonb `gorm:"column:latest_redeemed_rm"`
	RedeemRequired       bool           `gorm:"column:redeem_required"`
	LastRedeemTxnID      string         `gorm:"column:latest_redeem_txn_id" json:"last_redeem_txn_id"`
	StatusMessage        string         `gorm:"column:status_message" json:"status_message"`
	datastore.ModelWithTS
}

func (ReadMarkerEntity) TableName() string {
	return "read_markers"
}

func GetLatestReadMarker(ctx context.Context, clientID string) (*ReadMarker, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	rm := &ReadMarkerEntity{}
	err := db.First(rm, "client_id = ?", clientID).Error
	if err != nil {
		return nil, err
	}
	return rm.LatestRM, nil
}

func SaveLatestReadMarker(ctx context.Context, rm *ReadMarker, isCreate bool) error {
	db := datastore.GetStore().GetTransaction(ctx)
	rmEntity := &ReadMarkerEntity{}
	rmEntity.LatestRM = rm
	rmEntity.RedeemRequired = true
	if isCreate {
		err := db.Create(rmEntity).Error
		return err
	}
	err := db.Model(rmEntity).Updates(rmEntity).Error
	return err

}

func (rm *ReadMarkerEntity) UpdateStatus(ctx context.Context, status_message string, redeemTxn string) error {
	db := datastore.GetStore().GetTransaction(ctx)
	var err error
	rmUpdates := make(map[string]interface{})
	rmUpdates["latest_redeem_txn_id"] = redeemTxn
	rmUpdates["status_message"] = status_message
	rmUpdates["redeem_required"] = false
	latestRMBytes, err := json.Marshal(rm.LatestRM)
	if err != nil {
		return err
	}
	rmUpdates["latest_redeemed_rm"] = latestRMBytes
	err = db.Model(rm).Where("counter = ?", rm.LatestRM.ReadCounter).Updates(rmUpdates).Error
	return err
}
