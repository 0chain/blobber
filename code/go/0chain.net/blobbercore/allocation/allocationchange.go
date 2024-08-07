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
		ts common.Timestamp, allocationVersion int64, collector reference.QueryCollector) error
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
	ts common.Timestamp, allocationVersion int64) error {
	now := time.Now()
	collector := reference.NewCollector(len(cc.Changes))
	timeoutctx, cancel := context.WithTimeout(ctx, time.Second*60)
	defer cancel()
	eg, egCtx := errgroup.WithContext(timeoutctx)
	eg.SetLimit(10)
	for idx, change := range cc.Changes {
		select {
		case <-egCtx.Done():
			return egCtx.Err()
		default:
			changeIndex := idx
			eg.Go(func() error {
				change.AllocationID = cc.AllocationID
				changeProcessor := cc.AllocationChanges[changeIndex]
				return changeProcessor.ApplyChange(ctx, ts, allocationVersion, collector)
			})
		}
	}
	err := eg.Wait()
	if err != nil {
		return err
	}
	elapsedApplyChanges := time.Since(now)
	err = collector.Finalize(ctx, cc.AllocationID, allocationVersion)
	elapsedFinalize := time.Since(now) - elapsedApplyChanges
	logging.Logger.Info("ApplyChanges", zap.String("allocation_id", cc.AllocationID), zap.Duration("apply_changes", elapsedApplyChanges), zap.Duration("finalize", elapsedFinalize), zap.Int("changes", len(cc.Changes)))
	return err
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
func (a *AllocationChangeCollector) MoveToFilestore(ctx context.Context, allocationVersion int64) error {

	logging.Logger.Info("Move to filestore", zap.String("allocation_id", a.AllocationID))
	var (
		refs        []*reference.Ref
		useRefCache bool
		deletedRefs []*reference.Ref
	)
	refCache := reference.GetRefCache(a.AllocationID)
	defer reference.DeleteRefCache(a.AllocationID)
	if refCache != nil && refCache.AllocationVersion == allocationVersion {
		useRefCache = true
		refs = refCache.CreatedRefs
		deletedRefs = refCache.DeletedRefs
	} else if refCache != nil && refCache.AllocationVersion != allocationVersion {
		logging.Logger.Error("Ref cache is not valid", zap.String("allocation_id", a.AllocationID), zap.String("ref_cache_version", fmt.Sprintf("%d", refCache.AllocationVersion)), zap.String("allocation_version", fmt.Sprintf("%d", allocationVersion)))
	} else {
		logging.Logger.Error("Ref cache is nil", zap.String("allocation_id", a.AllocationID))
	}
	err := deleteFromFileStore(a.AllocationID, deletedRefs, useRefCache)
	if err != nil {
		return err
	}

	limitCh := make(chan struct{}, 12)
	wg := &sync.WaitGroup{}
	if !useRefCache {
		tx := datastore.GetStore().GetTransaction(ctx)
		err = tx.Model(&reference.Ref{}).Select("lookup_hash").Where("allocation_id=? AND allocation_version=? AND type=?", a.AllocationID, allocationVersion, reference.FILE).Find(&refs).Error
		if err != nil {
			logging.Logger.Error("Error while moving files to filestore", zap.Error(err))
			return err
		}
	}

	for _, ref := range refs {

		limitCh <- struct{}{}
		wg.Add(1)
		refLookupHash := ref.LookupHash
		go func(lookupHash string) {
			defer func() {
				<-limitCh
				wg.Done()
			}()
			logging.Logger.Info("Move to filestore", zap.String("lookup_hash", lookupHash))
			err := filestore.GetFileStore().MoveToFilestore(a.AllocationID, ref.LookupHash, filestore.VERSION)
			if err != nil {
				logging.Logger.Error(fmt.Sprintf("Error while moving file: %s", err.Error()))
			}

		}(refLookupHash)
	}

	wg.Wait()
	return nil
}

func deleteFromFileStore(allocationID string, deletedRefs []*reference.Ref, useRefCache bool) error {
	limitCh := make(chan struct{}, 12)
	wg := &sync.WaitGroup{}
	var results []*reference.Ref
	if useRefCache {
		results = deletedRefs
	}

	return datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		db := datastore.GetStore().GetTransaction(ctx)
		if !useRefCache {
			err := db.Model(&reference.Ref{}).Unscoped().Select("lookup_hash").
				Where("allocation_id=? AND type=? AND deleted_at is not NULL", allocationID, reference.FILE).
				Find(&results).Error
			if err != nil && err != gorm.ErrRecordNotFound {
				logging.Logger.Error("DeleteFromFileStore", zap.Error(err))
				return err
			}
		}

		for _, res := range results {
			limitCh <- struct{}{}
			wg.Add(1)
			resLookupHash := res.LookupHash
			go func(lookupHash string) {
				defer func() {
					<-limitCh
					wg.Done()
				}()

				err := filestore.GetFileStore().DeleteFromFilestore(allocationID, lookupHash,
					filestore.VERSION)
				if err != nil {
					logging.Logger.Error(fmt.Sprintf("Error while deleting file: %s", err.Error()),
						zap.String("validation_root", res.LookupHash))
				}
			}(resLookupHash)

		}
		wg.Wait()

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
