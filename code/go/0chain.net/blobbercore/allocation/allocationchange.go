package allocation

import (
	"context"
	"errors"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/constants"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	NewConnection        = 0
	InProgressConnection = 1
	CommittedConnection  = 2
	DeletedConnection    = 3
)

var OperationNotApplicable = common.NewError("operation_not_valid", "Not an applicable operation")

// AllocationChangeProcessor request transaction of file operation. it is president in postgres, and can be rebuilt for next http reqeust(eg CommitHandler)
type AllocationChangeProcessor interface {
	CommitToFileStore(ctx context.Context) error
	DeleteTempFile() error
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

func (change *AllocationChange) Save(ctx context.Context) error {
	db := datastore.GetStore().GetTransaction(ctx)

	return db.Save(change).Error
}

// GetAllocationChanges reload connection's changes in allocation from postgres.
//	1. update connection's status with NewConnection if connection_id is not found in postgres
//  2. mark as NewConnection if connection_id is marked as DeleteConnection
func GetAllocationChanges(ctx context.Context, connectionID, allocationID, clientID string) (*AllocationChangeCollector, error) {
	cc := &AllocationChangeCollector{}
	db := datastore.GetStore().GetTransaction(ctx)
	err := db.Where("connection_id = ? and allocation_id = ? and client_id = ? and status <> ?",
		connectionID,
		allocationID,
		clientID,
		DeletedConnection,
	).Preload("Changes").First(cc).Error

	if err == nil {
		cc.ComputeProperties()
		return cc, nil
	}

	// It is a bug when connetion_id was marked as DeletedConnection
	if errors.Is(err, gorm.ErrRecordNotFound) {
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
		return db.Create(cc).Error
	}

	return db.Save(cc).Error
}

// ComputeProperties unmarshal all ChangeProcesses from postgres
func (cc *AllocationChangeCollector) ComputeProperties() {
	cc.AllocationChanges = make([]AllocationChangeProcessor, 0, len(cc.Changes))
	for _, change := range cc.Changes {
		var acp AllocationChangeProcessor
		switch change.Operation {
		case constants.FileOperationInsert:
			acp = new(AddFileChanger)
		case constants.FileOperationUpdate:
			acp = new(UpdateFileChanger)
		case constants.FileOperationDelete:
			acp = new(DeleteFileChange)
		case constants.FileOperationRename:
			acp = new(RenameFileChange)
		case constants.FileOperationCopy:
			acp = new(CopyFileChange)
		case constants.FileOperationUpdateAttrs:
			acp = new(AttributesChange)
		}

		if acp == nil {
			continue // unknown operation (impossible case?)
		}

		if err := acp.Unmarshal(change.Input); err != nil { // error is not handled
			logging.Logger.Error("AllocationChangeCollector_unmarshal", zap.Error(err))
		}
		cc.AllocationChanges = append(cc.AllocationChanges, acp)
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
	for _, change := range a.AllocationChanges {
		err := change.CommitToFileStore(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *AllocationChangeCollector) DeleteChanges(ctx context.Context) {
	for _, change := range a.AllocationChanges {
		if err := change.DeleteTempFile(); err != nil {
			logging.Logger.Error("AllocationChangeProcessor_DeleteTempFile", zap.Error(err))
		}
	}
}
