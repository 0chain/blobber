package allocation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/constants"
	"golang.org/x/sync/errgroup"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	NewConnection        = 0
	InProgressConnection = 1
	CommittedConnection  = 2
	DeletedConnection    = 3
)

// AllocationChangeProcessor request transaction of file operation. it is president in postgres, and can be rebuilt for next http reqeust(eg CommitHandler)
type AllocationChangeProcessor interface {
	CommitToFileStore(ctx context.Context, mut *sync.Mutex) error
	DeleteTempFile() error
	ApplyChange(ctx context.Context,
		ts common.Timestamp, fileIDMeta map[string]string, collector reference.QueryCollector) error
	GetPath() []string
	Marshal() (string, error)
	Unmarshal(string) error
}

type AllocationChangeCollector struct {
	ID                string                      `gorm:"column:id;primaryKey"`
	AllocationID      string                      `gorm:"column:allocation_id;size:64;not null"`
	ClientID          string                      `gorm:"column:client_id;size:64;not null"`
	Size              int64                       `gorm:"column:size;not null;default:0"`
	Changes           []*AllocationChange         `gorm:"foreignKey:ConnectionID"`
	AllocationChanges []AllocationChangeProcessor `gorm:"-"`
	Status            int                         `gorm:"column:status;not null;default:0"`
	datastore.ModelWithTS
}

func (AllocationChangeCollector) TableName() string {
	return "allocation_connections"
}

func (ac *AllocationChangeCollector) BeforeCreate(tx *gorm.DB) error {
	ac.CreatedAt = time.Now()
	ac.UpdatedAt = ac.CreatedAt
	return nil
}

func (ac *AllocationChangeCollector) BeforeSave(tx *gorm.DB) error {
	ac.UpdatedAt = time.Now()
	return nil
}

type AllocationChange struct {
	ChangeID     int64                     `gorm:"column:id;primaryKey"`
	Size         int64                     `gorm:"column:size;not null;default:0"`
	Operation    string                    `gorm:"column:operation;size:20;not null"`
	ConnectionID string                    `gorm:"column:connection_id;size:64;not null"`
	Connection   AllocationChangeCollector `gorm:"foreignKey:ConnectionID"` // References allocation_connections(id)
	Input        string                    `gorm:"column:input"`
	FilePath     string                    `gorm:"-"`
	LookupHash   string                    `gorm:"column:lookup_hash;size:64"`
	AllocationID string                    `gorm:"-" json:"-"`
	datastore.ModelWithTS
}

func (AllocationChange) TableName() string {
	return "allocation_changes"
}

func (ac *AllocationChange) BeforeCreate(tx *gorm.DB) error {
	ac.CreatedAt = time.Now()
	ac.UpdatedAt = ac.CreatedAt
	return nil
}

func (ac *AllocationChange) BeforeSave(tx *gorm.DB) error {
	ac.UpdatedAt = time.Now()
	return nil
}

func (change *AllocationChange) Save(ctx context.Context) error {
	db := datastore.GetStore().GetTransaction(ctx)

	return db.Save(change).Error
}

func (change *AllocationChange) Update(ctx context.Context) error {
	db := datastore.GetStore().GetTransaction(ctx)
	return db.Table(change.TableName()).Where("connection_id = ? AND lookup_hash = ?", change.ConnectionID, change.LookupHash).Updates(map[string]interface{}{
		"size":       change.Size,
		"updated_at": time.Now(),
		"input":      change.Input,
	}).Error
}

func (change *AllocationChange) Create(ctx context.Context) error {
	db := datastore.GetStore().GetTransaction(ctx)
	return db.Create(change).Error
}

func ParseAffectedFilePath(input string) (string, error) {
	inputMap := make(map[string]interface{})
	err := json.Unmarshal([]byte(input), &inputMap)
	if err != nil {
		return "", err
	}
	if path, ok := inputMap["filepath"].(string); ok {
		return path, nil
	}
	return "", nil
}

func (change *AllocationChange) GetOrParseAffectedFilePath() (string, error) {

	// Check if change.FilePath has value
	if change.FilePath != "" {
		return change.FilePath, nil
	}
	filePath, err := ParseAffectedFilePath(change.Input)
	if err != nil {
		return "", err
	}
	change.FilePath = filePath
	return filePath, nil

}

// GetAllocationChanges reload connection's changes in allocation from postgres.
//  1. update connection's status with NewConnection if id is not found in postgres
//  2. mark as NewConnection if id is marked as DeleteConnection
func GetAllocationChanges(ctx context.Context, connectionID, allocationID, clientID string) (*AllocationChangeCollector, error) {
	cc := &AllocationChangeCollector{}
	db := datastore.GetStore().GetTransaction(ctx)
	err := db.Where("id = ? and allocation_id = ? and client_id = ? and status <> ?",
		connectionID,
		allocationID,
		clientID,
		DeletedConnection,
	).Preload("Changes").Take(cc).Error

	if err == nil {
		logging.Logger.Info("getAllocationChanges", zap.String("connection_id", connectionID), zap.Int("changes", len(cc.Changes)))
		cc.ComputeProperties()
		// Load connection Obj size from memory
		cc.Size = GetConnectionObjSize(connectionID)
		cc.Status = InProgressConnection
		return cc, nil
	}

	// It is a bug when connetion_id was marked as DeletedConnection
	if errors.Is(err, gorm.ErrRecordNotFound) {
		cc.ID = connectionID
		cc.AllocationID = allocationID
		cc.ClientID = clientID
		cc.Status = NewConnection
		return cc, nil
	}
	return nil, err
}

func GetConnectionObj(ctx context.Context, connectionID, allocationID, clientID string) (*AllocationChangeCollector, error) {
	cc := &AllocationChangeCollector{}
	db := datastore.GetStore().GetTransaction(ctx)
	err := db.Where("id = ? and allocation_id = ? and client_id = ? AND status <> ?",
		connectionID,
		allocationID,
		clientID,
		DeletedConnection,
	).Take(cc).Error

	if err == nil {
		return cc, nil
	}

	if err == gorm.ErrRecordNotFound {
		cc.ID = connectionID
		cc.AllocationID = allocationID
		cc.ClientID = clientID
		cc.Status = NewConnection
		err = cc.Save(ctx)
		if err != nil {
			return nil, err
		}
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

func (cc *AllocationChangeCollector) Create(ctx context.Context) error {
	db := datastore.GetStore().GetTransaction(ctx)
	cc.Status = NewConnection
	return db.Create(cc).Error
}

// ComputeProperties unmarshal all ChangeProcesses from postgres
func (cc *AllocationChangeCollector) ComputeProperties() {
	cc.AllocationChanges = make([]AllocationChangeProcessor, 0, len(cc.Changes))
	for _, change := range cc.Changes {
		var acp AllocationChangeProcessor
		switch change.Operation {
		case constants.FileOperationInsert:
			acp = new(UploadFileChanger)
		// case constants.FileOperationUpdate:
		// 	acp = new(UpdateFileChanger)
		case constants.FileOperationDelete:
			acp = new(DeleteFileChange)
		case constants.FileOperationRename:
			acp = new(RenameFileChange)
		case constants.FileOperationCopy:
			acp = new(CopyFileChange)
		case constants.FileOperationCreateDir:
			acp = new(NewDir)
		case constants.FileOperationMove:
			acp = new(MoveFileChange)
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

func (cc *AllocationChangeCollector) ApplyChanges(ctx context.Context,
	ts common.Timestamp, fileIDMeta map[string]string) error {
	collector := reference.NewCollector(len(cc.Changes))
	for idx, change := range cc.Changes {
		change.AllocationID = cc.AllocationID
		changeProcessor := cc.AllocationChanges[idx]
		err := changeProcessor.ApplyChange(ctx, ts, fileIDMeta, collector)
		if err != nil {
			return err
		}
	}
	return collector.Finalize(ctx)
}

func (a *AllocationChangeCollector) CommitToFileStore(ctx context.Context) error {
	// Limit can be configured at runtime, this number will depend on the number of active allocations
	eg, _ := errgroup.WithContext(ctx)
	eg.SetLimit(5)
	mut := &sync.Mutex{}
	for _, change := range a.AllocationChanges {
		allocChange := change
		eg.Go(func() error {
			return allocChange.CommitToFileStore(ctx, mut)
		})
	}
	logging.Logger.Info("Waiting for commit to filestore", zap.String("allocation_id", a.AllocationID))

	return eg.Wait()
}

func (a *AllocationChangeCollector) DeleteChanges(ctx context.Context) {
	for _, change := range a.AllocationChanges {
		if err := change.DeleteTempFile(); err != nil {
			logging.Logger.Error("AllocationChangeProcessor_DeleteTempFile", zap.Error(err))
		}
	}
}

type Result struct {
	LookupHash string
}

// TODO: Need to speed up this function
func (a *AllocationChangeCollector) MoveToFilestore(ctx context.Context) error {

	logging.Logger.Info("Move to filestore", zap.String("allocation_id", a.AllocationID))
	err := deleteFromFileStore(ctx, a.AllocationID)
	if err != nil {
		return err
	}
	var refs []*Result
	limitCh := make(chan struct{}, 25)
	wg := &sync.WaitGroup{}

	err = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		tx := datastore.GetStore().GetTransaction(ctx)
		err := tx.Model(&reference.Ref{}).Select("lookup_hash").Where("allocation_id=? AND is_precommit=? AND type=?", a.AllocationID, true, reference.FILE).
			FindInBatches(&refs, 100, func(tx *gorm.DB, batch int) error {

				for _, ref := range refs {

					limitCh <- struct{}{}
					wg.Add(1)

					go func(ref *Result) {
						defer func() {
							<-limitCh
							wg.Done()
						}()

						err := filestore.GetFileStore().MoveToFilestore(a.AllocationID, ref.LookupHash, filestore.VERSION)
						if err != nil {
							logging.Logger.Error(fmt.Sprintf("Error while moving file: %s", err.Error()))
						}

					}(ref)
				}

				return nil
			}).Error

		wg.Wait()

		if err != nil {
			logging.Logger.Error("Error while moving files to filestore", zap.Error(err))
			return err
		}

		return tx.Exec("UPDATE reference_objects SET is_precommit=? WHERE allocation_id=? AND is_precommit=? AND deleted_at is NULL", false, a.AllocationID, true).Error
	})
	return err
}

func deleteFromFileStore(ctx context.Context, allocationID string) error {
	limitCh := make(chan struct{}, 25)
	wg := &sync.WaitGroup{}
	var results []Result

	return datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		db := datastore.GetStore().GetTransaction(ctx)

		err := db.Model(&reference.Ref{}).Unscoped().Select("lookup_hash").
			Where("allocation_id=? AND is_precommit=? AND type=? AND deleted_at is not NULL", allocationID, true, reference.FILE).
			FindInBatches(&results, 100, func(tx *gorm.DB, batch int) error {

				for _, res := range results {
					limitCh <- struct{}{}
					wg.Add(1)

					go func(res Result) {
						defer func() {
							<-limitCh
							wg.Done()
						}()

						err := filestore.GetFileStore().DeleteFromFilestore(allocationID, res.LookupHash,
							filestore.VERSION)
						if err != nil {
							logging.Logger.Error(fmt.Sprintf("Error while deleting file: %s", err.Error()),
								zap.String("validation_root", res.LookupHash))
						}
					}(res)

				}
				return nil
			}).Error

		wg.Wait()
		if err != nil && err != gorm.ErrRecordNotFound {
			logging.Logger.Error("DeleteFromFileStore", zap.Error(err))
			return err
		}

		return db.Model(&reference.Ref{}).Unscoped().
			Delete(&reference.Ref{},
				"allocation_id = ? AND deleted_at IS NOT NULL",
				allocationID).Error
	})
}

// Note: We are also fetching refPath for srcPath in copy operation
func (a *AllocationChangeCollector) GetRootRef(ctx context.Context) (*reference.Ref, error) {
	paths := make([]string, 0)
	objTreePath := make([]string, 0)
	for _, change := range a.AllocationChanges {
		allPaths := change.GetPath()
		paths = append(paths, allPaths...)
		if len(allPaths) > 1 {
			objTreePath = append(objTreePath, allPaths[1])
		}
	}
	return reference.GetReferencePathFromPaths(ctx, a.AllocationID, paths, objTreePath)
}
