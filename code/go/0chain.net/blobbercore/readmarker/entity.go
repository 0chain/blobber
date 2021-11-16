package readmarker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"

	"gorm.io/datatypes"

	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

type AuthTicket struct {
	ClientID        string           `json:"client_id"`
	OwnerID         string           `json:"owner_id"`
	AllocationID    string           `json:"allocation_id"`
	FilePathHash    string           `json:"file_path_hash"`
	ActualFileHash  string           `json:"actual_file_hash"`
	FileName        string           `json:"file_name"`
	RefType         string           `json:"reference_type"`
	Expiration      common.Timestamp `json:"expiration"`
	Timestamp       common.Timestamp `json:"timestamp"`
	ReEncryptionKey string           `json:"re_encryption_key"`
	Signature       string           `json:"signature"`
	Encrypted       bool             `json:"encrypted"`
}

func (rm *AuthTicket) GetHashData() string {
	hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v:%v:%v:%v:%v", rm.AllocationID, rm.ClientID, rm.OwnerID, rm.FilePathHash, rm.FileName, rm.RefType, rm.ReEncryptionKey, rm.Expiration, rm.Timestamp, rm.ActualFileHash, rm.Encrypted)
	return hashData
}

func (authToken *AuthTicket) Verify(allocationObj *allocation.Allocation, clientID string) error {
	if authToken.AllocationID != allocationObj.ID {
		return common.NewError("invalid_parameters", "Invalid auth ticket. Allocation id mismatch")
	}
	if authToken.ClientID != clientID && len(authToken.ClientID) > 0 {
		return common.NewError("invalid_parameters", "Invalid auth ticket. Client ID mismatch")
	}
	if authToken.Expiration < authToken.Timestamp || authToken.Expiration < common.Now() {
		return common.NewError("invalid_parameters", "Invalid auth ticket. Expired ticket")
	}

	if authToken.OwnerID != allocationObj.OwnerID {
		return common.NewError("invalid_parameters", "Invalid auth ticket. Owner ID mismatch")
	}
	if authToken.Timestamp > (common.Now() + 2) {
		return common.NewError("invalid_parameters", "Invalid auth ticket. Timestamp in future")
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
	Suspend         int64            `gorm:"column:suspend" json:"suspend"`
	PayerID         string           `gorm:"column:payer_id" json:"payer_id"`
	AuthTicket      datatypes.JSON   `gorm:"column:auth_ticket" json:"auth_ticket"`
}

func (rm *ReadMarker) GetHashData() string {
	hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v", rm.AllocationID,
		rm.BlobberID, rm.ClientID, rm.ClientPublicKey, rm.OwnerID,
		rm.ReadCounter, rm.Timestamp)
	return hashData
}

type ReadMarkerEntity struct {
	LatestRM             *ReadMarker    `gorm:"embedded" json:"latest_read_marker,omitempty"`
	LatestRedeemedRMBlob datatypes.JSON `gorm:"column:latest_redeemed_rm"`
	RedeemRequired       bool           `gorm:"column:redeem_required"`
	LastRedeemTxnID      string         `gorm:"column:latest_redeem_txn_id" json:"last_redeem_txn_id"`
	StatusMessage        string         `gorm:"column:status_message" json:"status_message"`
	datastore.ModelWithTS
}

func (ReadMarkerEntity) TableName() string {
	return "read_markers"
}

func GetLatestReadMarkerEntity(ctx context.Context, clientID string) (*ReadMarkerEntity, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	rm := &ReadMarkerEntity{}
	err := db.First(rm, "client_id = ?", clientID).Error
	if err != nil {
		return nil, err
	}
	return rm, nil
}

func SaveLatestReadMarker(ctx context.Context, rm *ReadMarker, isCreate bool) error {
	var (
		db       = datastore.GetStore().GetTransaction(ctx)
		rmEntity = &ReadMarkerEntity{}
	)

	rmEntity.LatestRM = rm
	rmEntity.RedeemRequired = true
	rmEntity.LatestRedeemedRMBlob = datatypes.JSON("{}")

	if isCreate {
		return db.Create(rmEntity).Error
	}

	return db.Model(rmEntity).Updates(rmEntity).Error
}

// Sync read marker with 0chain to be sure its correct.
func (rm *ReadMarkerEntity) Sync(ctx context.Context) (err error) {

	var db = datastore.GetStore().GetTransaction(ctx)

	//////////////////////////////////////////////////////////////////////////////
	///////////////////////  Not using this map anywhere  ////////////////////////
	//////////////////////////////////////////////////////////////////////////////
	//////////////////////////////////////////////////////////////////////////////
	// var rmUpdates = make(map[string]interface{})								//
	// rmUpdates["latest_redeem_txn_id"] = "Synced from SC REST API"			//
	// rmUpdates["status_message"] = "sync"										//
	// rmUpdates["redeem_required"] = false										//
	//																			//
	// var latestRMBytes []byte													//
	// if latestRMBytes, err = json.Marshal(rm.LatestRM); err != nil {			//
	// 	return common.NewErrorf("rme_sync", "marshaling latest RM: %v", err)	//
	// }																		//
	// rmUpdates["latest_redeemed_rm"] = latestRMBytes							//
	//////////////////////////////////////////////////////////////////////////////
	//////////////////////////////////////////////////////////////////////////////

	// update local read pools cache from sharders
	var rps []*allocation.ReadPool
	rps, err = allocation.RequestReadPools(rm.LatestRM.ClientID,
		rm.LatestRM.AllocationID)
	if err != nil {
		return common.NewErrorf("rme_sync",
			"can't get read pools from sharders: %v", err)
	}

	// save the fresh read pools information
	err = allocation.SetReadPools(db, rm.LatestRM.ClientID,
		rm.LatestRM.AllocationID, rm.LatestRM.BlobberID, rps)
	if err != nil {
		return common.NewErrorf("rme_sync",
			"can't update read pools from sharders: %v", err)
	}

	return
}

// UpdateStatus updates read marker status and all related on successful
// redeeming.
func (rm *ReadMarkerEntity) UpdateStatus(ctx context.Context,
	rps []*allocation.ReadPool, txOutput, redeemTxn string) (err error) {

	var redeems []allocation.ReadPoolRedeem
	if err = json.Unmarshal([]byte(txOutput), &redeems); err != nil {
		Logger.Error("update read redeeming status: can't decode transaction"+
			" output", zap.Error(err))
		return common.NewErrorf("rme_update_status",
			"can't decode transaction output: %v", err)
	}

	var db = datastore.GetStore().GetTransaction(ctx)

	var rmUpdates = make(map[string]interface{})
	rmUpdates["latest_redeem_txn_id"] = redeemTxn
	rmUpdates["status_message"] = "success"
	rmUpdates["redeem_required"] = false

	var latestRMBytes []byte
	if latestRMBytes, err = json.Marshal(rm.LatestRM); err != nil {
		return common.NewErrorf("rme_update_status",
			"marshaling latest RM: %v", err)
	}
	if len(latestRMBytes) == 0 || string(latestRMBytes) == "" {
		latestRMBytes = []byte("{}")
	}
	rmUpdates["latest_redeemed_rm"] = latestRMBytes

	// the saving looses the numBlocks information
	err = db.Model(rm).
		Where("counter = ?", rm.LatestRM.ReadCounter).
		Updates(rmUpdates).Error
	if err != nil {
		return common.NewError("rme_update_status", err.Error())
	}

	// update cache using the transaction output
	allocation.SubReadRedeemed(rps, redeems)
	err = allocation.SetReadPools(db, rm.LatestRM.ClientID,
		rm.LatestRM.AllocationID, rm.LatestRM.BlobberID, rps)
	if err != nil {
		return common.NewErrorf("rme_update_status",
			"can't update local read pools cache: %v", err)
	}

	return
}
