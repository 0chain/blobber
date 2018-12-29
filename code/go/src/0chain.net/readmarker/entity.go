package readmarker

import (
	"context"
	"fmt"

	"0chain.net/common"
	"0chain.net/datastore"
	"0chain.net/encryption"
)

type ReadMarker struct {
	ClientID     string           `json:"client_id"`
	BlobberID    string           `json:"blobber_id"`
	AllocationID string           `json:"allocation_id"`
	OwnerID      string           `json:"owner_id"`
	Timestamp    common.Timestamp `json:"timestamp"`
	ReadCounter  int64            `json:"counter"`
	FilePath     string           `json:"filepath"`
	Signature    string           `json:"signature"`
}

type ReadMarkerEntity struct {
	LatestRM          *ReadMarker      `json:"latest_read_marker"`
	LastestRedeemedRM *ReadMarker      `json:"last_redeemed_read_marker"`
	LastRedeemTxnID   string           `json:"last_redeem_txn_id"`
	CreationDate      common.Timestamp `json:"creation_date"`
}

var readMarkerEntityMetaData *datastore.EntityMetadataImpl

/*Provider - entity provider for client object */
func Provider() datastore.Entity {
	t := &ReadMarkerEntity{}
	t.CreationDate = common.Now()
	return t
}

func SetupEntity(store datastore.Store) {
	readMarkerEntityMetaData = datastore.MetadataProvider()
	readMarkerEntityMetaData.Name = "rm"
	readMarkerEntityMetaData.DB = "rm"
	readMarkerEntityMetaData.Provider = Provider
	readMarkerEntityMetaData.Store = store

	datastore.RegisterEntityMetadata("rm", readMarkerEntityMetaData)
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
	hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v", rm.AllocationID, rm.BlobberID, rm.ClientID, rm.OwnerID, rm.FilePath, rm.ReadCounter, rm.Timestamp)
	return hashData
}
