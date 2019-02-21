package writemarker

import (
	"context"
	"fmt"

	"0chain.net/allocation"
	"0chain.net/common"
	"0chain.net/datastore"
	"0chain.net/encryption"
	"0chain.net/hashmapstore"
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

func (wm *WriteMarker) GetHashData() string {
	hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v", wm.AllocationRoot, wm.PreviousAllocationRoot, wm.AllocationID, wm.BlobberID, wm.ClientID, wm.Size, wm.Timestamp)
	return hashData
}

type WriteMarkerStatus int

const (
	Accepted  WriteMarkerStatus = 0
	Committed WriteMarkerStatus = 1
	Failed    WriteMarkerStatus = 2
)

type WriteMarkerEntity struct {
	Version         string                         `json:"version"`
	PrevWM          string                         `json:"prev_write_marker"`
	WM              *WriteMarker                   `json:"write_marker"`
	Status          WriteMarkerStatus              `json:"status"`
	StatusMessage   string                         `json:"status_message"`
	ReedeemRetries  int64                          `json:"redeem_retries"`
	CloseTxnID      string                         `json:"close_txn_id"`
	CreationDate    common.Timestamp               `json:"creation_date"`
	Changes         []*allocation.AllocationChange `json:"changes"`
	DirStructure    map[string][]byte              `json:"dir_struct,omitempty"`
	ClientPublicKey string                         `json:"client_key"`
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

func (wm *WriteMarkerEntity) WriteAllocationDirStructure(ctx context.Context) error {
	dbStore := hashmapstore.NewStore()
	allocationChangeCollector := allocation.AllocationChangeCollectorProvider().(*allocation.AllocationChangeCollector)
	allocationChangeCollector.AllocationID = wm.WM.AllocationID
	allocationChangeCollector.ConnectionID = ""
	curWM := wm
	curDB := dbStore.DB
	changes := make([]*allocation.AllocationChange, 0)
	changes = append(curWM.Changes, changes...)
	for len(curWM.PrevWM) > 0 {
		prevWMEntity := wm.GetEntityMetadata().Instance().(*WriteMarkerEntity)
		err := prevWMEntity.Read(ctx, curWM.PrevWM)
		if err != nil {
			return err
		}
		if prevWMEntity.DirStructure != nil {
			curDB = prevWMEntity.DirStructure
			break
		}
		curWM = prevWMEntity
		changes = append(curWM.Changes, changes...)
	}
	dbStore.DB = curDB
	allocationChangeCollector.Changes = changes

	_, err := allocationChangeCollector.ApplyChanges(ctx, nil, dbStore, wm.GetKey())
	if err != nil {
		return err
	}
	wm.DirStructure = dbStore.DB
	return nil
}
