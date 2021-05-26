package allocation

import (
	"context"
	"errors"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	INSERT_OPERATION       = "insert"
	DELETE_OPERATION       = "delete"
	UPDATE_OPERATION       = "update"
	RENAME_OPERATION       = "rename"
	COPY_OPERATION         = "copy"
	UPDATE_ATTRS_OPERATION = "update_attrs"
)

const (
	NewConnection        = 0
	InProgressConnection = 1
	CommittedConnection  = 2
	DeletedConnection    = 3
)

var OperationNotApplicable = common.NewError("operation_not_valid", "Not an applicable operation")

type AllocationChangeProcessor interface {
	CommitToFileStore(ctx context.Context) error
	DeleteTempFile() error
	ProcessChange(ctx context.Context, change *AllocationChange, allocationRoot string) (*reference.Ref, error)
	Marshal() (string, error)
	Unmarshal(string) error
}

type IAllocationChangeCollector interface {
	TableName() string
	AddChange(allocationChange *AllocationChange, changeProcessor AllocationChangeProcessor)
	Save(ctx context.Context) error
	ComputeProperties()
	ApplyChanges(ctx context.Context, allocationRoot string) error
	CommitToFileStore(ctx context.Context) error
	DeleteChanges(ctx context.Context)
	GetAllocationID() string
	GetConnectionID() string
	GetClientID() string
	GetSize() int64
	SetSize(size int64)
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
	err := db.Where(&AllocationChangeCollector{
		ConnectionID: connectionID,
		AllocationID: allocationID,
		ClientID:     clientID,
	}).Not(&AllocationChangeCollector{
		Status: DeletedConnection,
	}).Preload("Changes").First(cc).Error

	if err == nil {
		cc.ComputeProperties()
		return cc, nil
	}

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
		err := db.Create(cc).Error
		return err
	} else {
		err := db.Save(cc).Error
		return err
	}
}

func (cc *AllocationChangeCollector) ComputeProperties() {
	cc.AllocationChanges = make([]AllocationChangeProcessor, 0, len(cc.Changes))
	for _, change := range cc.Changes {
		var acp AllocationChangeProcessor
		switch change.Operation {
		case INSERT_OPERATION:
			acp = new(NewFileChange)
		case UPDATE_OPERATION:
			acp = new(UpdateFileChange)
		case DELETE_OPERATION:
			acp = new(DeleteFileChange)
		case RENAME_OPERATION:
			acp = new(RenameFileChange)
		case COPY_OPERATION:
			acp = new(CopyFileChange)
		case UPDATE_ATTRS_OPERATION:
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

func (cc *AllocationChangeCollector) GetAllocationID() string {
	return cc.AllocationID
}

func (cc *AllocationChangeCollector) GetConnectionID() string {
	return cc.ConnectionID
}

func (cc *AllocationChangeCollector) GetClientID() string {
	return cc.ClientID
}

func (cc *AllocationChangeCollector) GetSize() int64 {
	return cc.Size
}

func (cc *AllocationChangeCollector) SetSize(size int64) {
	cc.Size = size
}
