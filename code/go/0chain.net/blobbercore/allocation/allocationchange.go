package allocation

import (
	"context"

	"0chain.net/blobbercore/datastore"
	"0chain.net/blobbercore/filestore"
	"0chain.net/blobbercore/reference"
	"0chain.net/core/common"
	"github.com/jinzhu/gorm"
)

const (
	INSERT_OPERATION = "insert"
	DELETE_OPERATION = "delete"
	UPDATE_OPERATION = "update"
)

const (
	NewConnection        = 0
	InProgressConnection = 1
	CommittedConnection  = 2
)

type AllocationChangeProcessor interface {
	ProcessChange(ctx context.Context, change *AllocationChange, allocationRoot string) (*reference.Ref, error)
	Marshal() (string, error)
	Unmarshal(string) error
}

type AllocationChangeCollector struct {
	ConnectionID      string                      `gorm:"column:connection_id;primary_key"`
	AllocationID      string                      `gorm:"column:allocation_id"`
	ClientID          string                      `gorm:"column:client_id"`
	Size              int64                       `gorm:"column:size"`
	Changes           []*AllocationChange         `gorm:"ForeignKey:connection_id;AssociationForeignKey:connection_id"`
	AllocationChanges []AllocationChangeProcessor `gorm:"-"`
	Status            int                         `gorm:"column:status"`
	datastore.ModelWithTS
}

func (AllocationChangeCollector) TableName() string {
	return "allocation_connections"
}

type AllocationChange struct {
	ChangeID     int64  `gorm:"column:id;primary_key"`
	Size         int64  `gorm:"column:size"`
	Operation    string `gorm:"column:operation"`
	ConnectionID string `gorm:"column:connection_id"`
	Input        string `gorm:"column:input"`
	datastore.ModelWithTS
}

func (AllocationChange) TableName() string {
	return "allocation_changes"
}

func GetAllocationChanges(ctx context.Context, connectionID string, allocationID string, clientID string) (*AllocationChangeCollector, error) {
	cc := &AllocationChangeCollector{}
	db := datastore.GetStore().GetTransaction(ctx)
	err := db.Where(&AllocationChangeCollector{ConnectionID: connectionID, AllocationID: allocationID, ClientID: clientID}).Preload("Changes").First(cc).Error

	if err == nil {
		cc.ComputeProperties()
		return cc, nil
	}

	if err != nil && gorm.IsRecordNotFoundError(err) {
		cc.ConnectionID = connectionID
		cc.AllocationID = allocationID
		cc.ClientID = clientID
		cc.Status = NewConnection
		return cc, nil
	}
	return nil, err
}

func (cc *AllocationChangeCollector) AddChange(allocationChange *AllocationChange, changeProcessor AllocationChangeProcessor) {
	cc.AllocationChanges = append(cc.AllocationChanges, changeProcessor)
	allocationChange.Input, _ = changeProcessor.Marshal()
	cc.Changes = append(cc.Changes, allocationChange)
}

func (cc *AllocationChangeCollector) Save(ctx context.Context) error {

	db := datastore.GetStore().GetTransaction(ctx)
	if cc.Status == NewConnection {
		cc.Status = InProgressConnection
		err := db.Create(cc).Error
		return err
	} else {
		err := db.Save(cc).Error
		return err
	}
}

func (cc *AllocationChangeCollector) ComputeProperties() {
	cc.AllocationChanges = make([]AllocationChangeProcessor, len(cc.Changes))
	for idx, change := range cc.Changes {
		if change.Operation == INSERT_OPERATION {
			nfc := &NewFileChange{}
			nfc.Unmarshal(change.Input)
			cc.AllocationChanges[idx] = nfc
		} else if change.Operation == UPDATE_OPERATION {
			ufc := &UpdateFileChange{}
			ufc.Unmarshal(change.Input)
			cc.AllocationChanges[idx] = ufc
		} else if change.Operation == DELETE_OPERATION {
			dfc := &DeleteFileChange{}
			dfc.Unmarshal(change.Input)
			cc.AllocationChanges[idx] = dfc
		}
	}
}

func (cc *AllocationChangeCollector) ApplyChanges(ctx context.Context, allocationRoot string) error {
	for idx, change := range cc.Changes {
		changeProcessor := cc.AllocationChanges[idx]
		_, err := changeProcessor.ProcessChange(ctx, change, allocationRoot)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *AllocationChangeCollector) CommitToFileStore(ctx context.Context) error {
	for idx, change := range a.Changes {
		if change.Operation == INSERT_OPERATION {
			nfch := a.AllocationChanges[idx].(*NewFileChange)
			fileInputData := &filestore.FileInputData{}
			fileInputData.Name = nfch.Filename
			fileInputData.Path = nfch.Path
			fileInputData.Hash = nfch.Hash
			_, err := filestore.GetFileStore().CommitWrite(a.AllocationID, fileInputData, a.ConnectionID)
			if err != nil {
				return common.NewError("file_store_error", "Error committing to file store. "+err.Error())
			}
			if nfch.ThumbnailSize > 0 {
				fileInputData := &filestore.FileInputData{}
				fileInputData.Name = nfch.ThumbnailFilename
				fileInputData.Path = nfch.Path
				fileInputData.Hash = nfch.ThumbnailHash
				_, err := filestore.GetFileStore().CommitWrite(a.AllocationID, fileInputData, a.ConnectionID)
				if err != nil {
					return common.NewError("file_store_error", "Error committing to file store. "+err.Error())
				}
			}
		} else if change.Operation == UPDATE_OPERATION {
			nfch := a.AllocationChanges[idx].(*UpdateFileChange)
			fileInputData := &filestore.FileInputData{}
			fileInputData.Name = nfch.Filename
			fileInputData.Path = nfch.Path
			fileInputData.Hash = nfch.Hash
			_, err := filestore.GetFileStore().CommitWrite(a.AllocationID, fileInputData, a.ConnectionID)
			if err != nil {
				return common.NewError("file_store_error", "Error committing to file store. "+err.Error())
			}
			if nfch.ThumbnailSize > 0 {
				fileInputData := &filestore.FileInputData{}
				fileInputData.Name = nfch.ThumbnailFilename
				fileInputData.Path = nfch.Path
				fileInputData.Hash = nfch.ThumbnailHash
				_, err := filestore.GetFileStore().CommitWrite(a.AllocationID, fileInputData, a.ConnectionID)
				if err != nil {
					return common.NewError("file_store_error", "Error committing to file store. "+err.Error())
				}
			}
		}
	}
	return nil
}

func (a *AllocationChangeCollector) DeleteChanges(ctx context.Context) error {
	for idx, change := range a.Changes {
		if change.Operation == INSERT_OPERATION {
			nfch := a.AllocationChanges[idx].(*NewFileChange)
			fileInputData := &filestore.FileInputData{}
			fileInputData.Name = nfch.Filename
			fileInputData.Path = nfch.Path
			fileInputData.Hash = nfch.Hash
			filestore.GetFileStore().DeleteTempFile(a.AllocationID, fileInputData, a.ConnectionID)
			if nfch.ThumbnailSize > 0 {
				fileInputData := &filestore.FileInputData{}
				fileInputData.Name = nfch.ThumbnailFilename
				fileInputData.Path = nfch.Path
				fileInputData.Hash = nfch.ThumbnailHash
				filestore.GetFileStore().DeleteTempFile(a.AllocationID, fileInputData, a.ConnectionID)
			}
		} else if change.Operation == UPDATE_OPERATION {
			nfch := a.AllocationChanges[idx].(*UpdateFileChange)
			fileInputData := &filestore.FileInputData{}
			fileInputData.Name = nfch.Filename
			fileInputData.Path = nfch.Path
			fileInputData.Hash = nfch.Hash
			filestore.GetFileStore().DeleteTempFile(a.AllocationID, fileInputData, a.ConnectionID)
			if nfch.ThumbnailSize > 0 {
				fileInputData := &filestore.FileInputData{}
				fileInputData.Name = nfch.ThumbnailFilename
				fileInputData.Path = nfch.Path
				fileInputData.Hash = nfch.ThumbnailHash
				filestore.GetFileStore().DeleteTempFile(a.AllocationID, fileInputData, a.ConnectionID)
			}
		}
	}

	return nil
}
