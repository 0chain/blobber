package readmarker

import (
	"context"
	"fmt"

	"0chain.net/allocation"
	"0chain.net/common"
	"0chain.net/datastore"
	"0chain.net/encryption"
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
	ClientID        string           `json:"client_id"`
	ClientPublicKey string           `json:"client_public_key"`
	BlobberID       string           `json:"blobber_id"`
	AllocationID    string           `json:"allocation_id"`
	OwnerID         string           `json:"owner_id"`
	Timestamp       common.Timestamp `json:"timestamp"`
	ReadCounter     int64            `json:"counter"`
	Signature       string           `json:"signature"`
}

type ReadMarkerStatus struct {
	LastestRedeemedRM *ReadMarker `json:"last_redeemed_read_marker"`
	LastRedeemTxnID   string      `json:"last_redeem_txn_id"`
	StatusMessage     string      `json:"status_message"`
}

type ReadMarkerEntity struct {
	LatestRM *ReadMarker `json:"latest_read_marker,omitempty"`
}

var readMarkerEntityMetaData *datastore.EntityMetadataImpl
var readMarkerStatusEntityMetaData *datastore.EntityMetadataImpl

/*Provider - entity provider for client object */
func Provider() datastore.Entity {
	t := &ReadMarkerEntity{}
	return t
}

func SetupEntity(store datastore.Store) {
	readMarkerEntityMetaData = datastore.MetadataProvider()
	readMarkerEntityMetaData.Name = "rm"
	readMarkerEntityMetaData.DB = "rm"
	readMarkerEntityMetaData.Provider = Provider
	readMarkerEntityMetaData.Store = store

	readMarkerStatusEntityMetaData = datastore.MetadataProvider()
	readMarkerStatusEntityMetaData.Name = "rm_status"
	readMarkerStatusEntityMetaData.DB = "rm_status"
	readMarkerStatusEntityMetaData.Provider = Provider
	readMarkerStatusEntityMetaData.Store = store

	datastore.RegisterEntityMetadata("rm", readMarkerEntityMetaData)
	datastore.RegisterEntityMetadata("rm_status", readMarkerStatusEntityMetaData)
}

func (rm *ReadMarkerEntity) GetLatestReadMarker(ctx context.Context, clientID string, blobberID string) error {
	newRM := &ReadMarker{ClientID: clientID, BlobberID: blobberID}
	rm.LatestRM = newRM
	err := rm.Read(ctx, rm.GetKey())
	return err
}

func (rm *ReadMarkerEntity) GetEntityMetadata() datastore.EntityMetadata {
	return readMarkerEntityMetaData
}
func (rm *ReadMarkerEntity) SetKey(key datastore.Key) {
	//wm.ID = datastore.ToString(key)
}
func (rm *ReadMarkerEntity) GetKey() datastore.Key {
	return datastore.ToKey(readMarkerEntityMetaData.GetDBName() + ":" + encryption.Hash(rm.LatestRM.ClientID+rm.LatestRM.BlobberID))
}
func (rm *ReadMarkerEntity) Read(ctx context.Context, key datastore.Key) error {
	return readMarkerEntityMetaData.GetStore().Read(ctx, key, rm)
}
func (rm *ReadMarkerEntity) Write(ctx context.Context) error {
	return readMarkerEntityMetaData.GetStore().Write(ctx, rm)
}
func (rm *ReadMarkerEntity) Delete(ctx context.Context) error {
	return nil
}

func (rm *ReadMarker) GetHashData() string {
	hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v", rm.AllocationID, rm.BlobberID, rm.ClientID, rm.ClientPublicKey, rm.OwnerID, rm.ReadCounter, rm.Timestamp)
	return hashData
}

func (rmstatus *ReadMarkerStatus) GetEntityMetadata() datastore.EntityMetadata {
	return readMarkerStatusEntityMetaData
}
func (rmstatus *ReadMarkerStatus) SetKey(key datastore.Key) {
	//wm.ID = datastore.ToString(key)
}
func (rmstatus *ReadMarkerStatus) GetKey() datastore.Key {
	return datastore.ToKey(readMarkerStatusEntityMetaData.GetDBName() + ":" + encryption.Hash(rmstatus.LastestRedeemedRM.ClientID+rmstatus.LastestRedeemedRM.BlobberID))
}
func (rmstatus *ReadMarkerStatus) Read(ctx context.Context, key datastore.Key) error {
	return readMarkerStatusEntityMetaData.GetStore().Read(ctx, key, rmstatus)
}
func (rmstatus *ReadMarkerStatus) Write(ctx context.Context) error {
	return readMarkerStatusEntityMetaData.GetStore().Write(ctx, rmstatus)
}
func (rmstatus *ReadMarkerStatus) Delete(ctx context.Context) error {
	return nil
}
