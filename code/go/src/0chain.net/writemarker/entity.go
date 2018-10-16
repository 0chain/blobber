package writemarker

import (
	"context"

	"0chain.net/common"
	"0chain.net/datastore"
	"0chain.net/encryption"
)

type WriteMarker struct {
	DataID              string           `json:"data_id"`
	MerkleRoot          string           `json:"merkle_root"`
	IntentTransactionID string           `json:"intent_tx_id"`
	BlobberID           string           `json:"blobber_id"`
	Timestamp           common.Timestamp `json:"timestamp"`
	ClientID            string           `json:"client_id"`
	Signature           string           `json:"signature"`
}

type WriteMarkerStatus int

const (
	Accepted  WriteMarkerStatus = 0
	Committed WriteMarkerStatus = 1
	Failed    WriteMarkerStatus = 2
)

type WriteMarkerEntity struct {
	ID            string            `json:"id"`
	Version       string            `json:"version"`
	AllocationID  string            `json:"allocation_id"`
	WM            *WriteMarker      `json:"write_marker"`
	MerkleRoot    string            `json:"merkle_root"`
	ContentHash   string            `json:"content_hash"`
	Status        WriteMarkerStatus `json:"status"`
	StatusMessage string            `json:"status_message"`
}

var writeMarkerEntityMetaData *datastore.EntityMetadataImpl

/*Provider - entity provider for client object */
func Provider() datastore.Entity {
	t := &WriteMarkerEntity{}
	t.Version = "1.0"
	return t
}

func SetupWMEntity(store datastore.Store) {
	writeMarkerEntityMetaData = datastore.MetadataProvider()
	writeMarkerEntityMetaData.Name = "wm"
	writeMarkerEntityMetaData.DB = "wmdb"
	writeMarkerEntityMetaData.Provider = Provider
	writeMarkerEntityMetaData.Store = store

	datastore.RegisterEntityMetadata("wm", writeMarkerEntityMetaData)
}

func (wm *WriteMarkerEntity) GetEntityMetadata() datastore.EntityMetadata {
	return writeMarkerEntityMetaData
}
func (wm *WriteMarkerEntity) SetKey(key datastore.Key) {
	wm.ID = datastore.ToString(key)
}
func (wm *WriteMarkerEntity) GetKey() datastore.Key {
	return datastore.ToKey(encryption.Hash(wm.AllocationID + wm.WM.DataID))
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
