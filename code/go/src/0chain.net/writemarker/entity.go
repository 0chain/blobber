package writemarker

import (
	"context"
	"fmt"

	"0chain.net/common"
	"0chain.net/datastore"
	"0chain.net/encryption"
)

type WriteMarker struct {
	AllocationRoot         string           `json:"allocation_root"`
	PreviousAllocationRoot string           `json:"prev_allocation_root"`
	AllocationID           string           `json:"allocation_id"`
	Size                   int64            `json:"size"`
	BlobberID              string           `json:"blobber_id"`
	Timestamp              common.Timestamp `json:"timestamp"`
	ClientID               string           `json:"client_id"`
	Signature              string           `json:"signature"`
}

type WriteMarkerStatus int

const (
	Accepted  WriteMarkerStatus = 0
	Committed WriteMarkerStatus = 1
	Failed    WriteMarkerStatus = 2
)

type WriteMarkerEntity struct {
	Version        string            `json:"version"`
	PrevWM         string            `json:"prev_write_marker"`
	WM             *WriteMarker      `json:"write_marker"`
	Status         WriteMarkerStatus `json:"status"`
	StatusMessage  string            `json:"status_message"`
	ReedeemRetries int64             `json:"redeem_retries"`
	CloseTxnID     string            `json:"close_txn_id"`
	CreationDate   common.Timestamp  `json:"creation_date"`
}

var writeMarkerEntityMetaData *datastore.EntityMetadataImpl

/*Provider - entity provider for client object */
func Provider() datastore.Entity {
	t := &WriteMarkerEntity{}
	t.Version = "1.0"
	t.CreationDate = common.Now()
	return t
}

func SetupEntity(store datastore.Store) {
	writeMarkerEntityMetaData = datastore.MetadataProvider()
	writeMarkerEntityMetaData.Name = "wm"
	writeMarkerEntityMetaData.DB = "wm"
	writeMarkerEntityMetaData.Provider = Provider
	writeMarkerEntityMetaData.Store = store

	datastore.RegisterEntityMetadata("wm", writeMarkerEntityMetaData)
}

func (wm *WriteMarkerEntity) GetEntityMetadata() datastore.EntityMetadata {
	return writeMarkerEntityMetaData
}
func (wm *WriteMarkerEntity) SetKey(key datastore.Key) {
	//wm.ID = datastore.ToString(key)
}
func (wm *WriteMarkerEntity) GetKey() datastore.Key {
	return datastore.ToKey(writeMarkerEntityMetaData.GetDBName() + ":" + encryption.Hash(wm.WM.AllocationID+wm.WM.AllocationRoot))
}
func (wm *WriteMarkerEntity) Read(ctx context.Context, key datastore.Key) error {
	return writeMarkerEntityMetaData.GetStore().Read(ctx, key, wm)
}
func (wm *WriteMarkerEntity) Write(ctx context.Context) error {
	return writeMarkerEntityMetaData.GetStore().Write(ctx, wm)
}
func (wm *WriteMarkerEntity) Delete(ctx context.Context) error {
	return nil
}

func (wm *WriteMarker) GetHashData() string {
	hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v", wm.AllocationRoot, wm.PreviousAllocationRoot, wm.AllocationID, wm.BlobberID, wm.ClientID, wm.Size, wm.Timestamp)
	return hashData
}
