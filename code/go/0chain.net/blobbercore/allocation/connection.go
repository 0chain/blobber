package allocation

import (
	"context"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

var (
	// ConnectionObjCleanInterval start to clean the connectionObjMap
	ConnectionObjCleanInterval = 10 * time.Minute
	// ConnectionObjTimout after which connectionObj entry should be invalid
	ConnectionObjTimeout = 10 * time.Minute
)

var (
	connectionProcessor = make(map[string]*ConnectionProcessor)
	connectionObjMutex  sync.RWMutex
)

type ConnectionProcessor struct {
	Size          int64
	UpdatedAt     time.Time
	AllocationObj *Allocation
	changes       map[string]*ConnectionChange
	ClientID      string
}

type ConnectionChange struct {
	hasher       *filestore.CommitHasher
	baseChanger  *BaseFileChanger
	existingRef  *reference.Ref
	processError error
	ProcessChan  chan FileCommand
	wg           sync.WaitGroup
	isFinalized  bool
}

func CreateConnectionChange(connectionID, pathHash string) *ConnectionChange {
	connectionObjMutex.Lock()
	defer connectionObjMutex.Unlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		connectionObj = &ConnectionProcessor{
			UpdatedAt: time.Now(),
			changes:   make(map[string]*ConnectionChange),
		}
		connectionProcessor[connectionID] = connectionObj
	}
	connChange := &ConnectionChange{
		ProcessChan: make(chan FileCommand, 2),
		wg:          sync.WaitGroup{},
	}
	connectionObj.changes[pathHash] = connChange
	connChange.wg.Add(1)
	go func() {
		processCommand(connChange.ProcessChan, connectionObj.AllocationObj, connectionID, connectionObj.ClientID, pathHash)
		connChange.wg.Done()
	}()
	return connChange
}

func GetConnectionChange(connectionID, pathHash string) *ConnectionChange {
	connectionObjMutex.RLock()
	defer connectionObjMutex.RUnlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		return nil
	}
	return connectionObj.changes[pathHash]
}

func GetFileChanger(connectionID, pathHash string) *BaseFileChanger {
	connectionObjMutex.RLock()
	defer connectionObjMutex.RUnlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		return nil
	}
	if connectionObj.changes[pathHash] == nil {
		return nil
	}
	return connectionObj.changes[pathHash].baseChanger
}

func SaveFileChanger(connectionID string, fileChanger *BaseFileChanger) error {
	connectionObjMutex.Lock()
	defer connectionObjMutex.Unlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		return common.NewError("connection_not_found", "connection not found")
	}
	if connectionObj.changes[fileChanger.PathHash] == nil {
		return common.NewError("connection_change_not_found", "connection change not found")
	}
	connectionObj.changes[fileChanger.PathHash].baseChanger = fileChanger
	return nil
}

func SaveExistingRef(connectionID, pathHash string, existingRef *reference.Ref) error {
	connectionObjMutex.Lock()
	defer connectionObjMutex.Unlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		return common.NewError("connection_not_found", "connection not found")
	}
	if connectionObj.changes[pathHash] == nil {
		return common.NewError("connection_change_not_found", "connection change not found")
	}
	connectionObj.changes[pathHash].existingRef = existingRef
	return nil
}

func GetExistingRef(connectionID, pathHash string) *reference.Ref {
	connectionObjMutex.RLock()
	defer connectionObjMutex.RUnlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		return nil
	}
	if connectionObj.changes[pathHash] == nil {
		return nil
	}
	return connectionObj.changes[pathHash].existingRef
}

func SetFinalized(connectionID, pathHash string, cmd FileCommand) error {
	connectionObjMutex.Lock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		connectionObjMutex.Unlock()
		return common.NewError("connection_not_found", "connection not found")
	}
	connChange := connectionObj.changes[pathHash]
	if connChange.isFinalized {
		connectionObjMutex.Unlock()
		return common.NewError("connection_change_finalized", "connection change finalized")
	}
	connChange.isFinalized = true
	connectionObjMutex.Unlock()
	logging.Logger.Info("Sending final command", zap.String("connection_id", connectionID), zap.String("path", cmd.GetPath()))
	connChange.ProcessChan <- cmd
	close(connChange.ProcessChan)
	connChange.wg.Wait()
	return GetError(connectionID, pathHash)
}

func SendCommand(connectionID, pathHash string, cmd FileCommand) error {
	connectionObjMutex.RLock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		connectionObjMutex.RUnlock()
		return common.NewError("connection_not_found", "connection not found")
	}
	connChange := connectionObj.changes[pathHash]
	if connChange == nil {
		connectionObjMutex.RUnlock()
		return common.NewError("connection_change_not_found", "connection change not found")
	}
	if connChange.processError != nil {
		connectionObjMutex.RUnlock()
		return connChange.processError
	}
	if connChange.isFinalized {
		connectionObjMutex.RUnlock()
		return common.NewError("connection_change_finalized", "connection change finalized")
	}
	connectionObjMutex.RUnlock()
	logging.Logger.Info("Sending command", zap.String("connection_id", connectionID), zap.String("path", cmd.GetPath()))
	connChange.ProcessChan <- cmd
	return nil
}

func GetConnectionProcessor(connectionID string) *ConnectionProcessor {
	connectionObjMutex.RLock()
	defer connectionObjMutex.RUnlock()
	return connectionProcessor[connectionID]
}

func CreateConnectionProcessor(connectionID string) *ConnectionProcessor {
	connectionObjMutex.Lock()
	defer connectionObjMutex.Unlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		connectionObj = &ConnectionProcessor{
			UpdatedAt: time.Now(),
			changes:   make(map[string]*ConnectionChange),
		}
		connectionProcessor[connectionID] = connectionObj
	}
	return connectionObj
}

func SetError(connectionID, pathHash string, err error) {
	connectionObjMutex.Lock()
	defer connectionObjMutex.Unlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		return
	}
	connChange := connectionObj.changes[pathHash]
	connChange.processError = err
	drainChan(connChange.ProcessChan) // drain the channel so that the no commands are blocked
}

func GetError(connectionID, pathHash string) error {
	connectionObjMutex.RLock()
	defer connectionObjMutex.RUnlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		return nil
	}
	connChange := connectionObj.changes[pathHash]
	if connChange == nil {
		return nil
	}
	return connChange.processError
}

// GetConnectionObjSize gets the connection size from the memory
func GetConnectionObjSize(connectionID string) int64 {
	connectionObjMutex.RLock()
	defer connectionObjMutex.RUnlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		return 0
	}
	return connectionObj.Size
}

// UpdateConnectionObjSize updates the connection size by addSize in memory
func UpdateConnectionObjSize(connectionID string, addSize int64) {
	connectionObjMutex.Lock()
	defer connectionObjMutex.Unlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		connectionProcessor[connectionID] = &ConnectionProcessor{
			Size:      addSize,
			UpdatedAt: time.Now(),
			changes:   make(map[string]*ConnectionChange),
		}
		return
	}

	connectionObj.Size = connectionObj.Size + addSize
	connectionObj.UpdatedAt = time.Now()
}

func GetHasher(connectionID, pathHash string) *filestore.CommitHasher {
	connectionObjMutex.RLock()
	defer connectionObjMutex.RUnlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		return nil
	}
	if connectionObj.changes[pathHash] == nil {
		return nil
	}
	return connectionObj.changes[pathHash].hasher
}

func UpdateConnectionObjWithHasher(connectionID, pathHash string, hasher *filestore.CommitHasher) {
	connectionObjMutex.Lock()
	defer connectionObjMutex.Unlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		connectionObj = &ConnectionProcessor{
			UpdatedAt: time.Now(),
			changes:   make(map[string]*ConnectionChange),
		}
		connectionProcessor[connectionID] = connectionObj
	}
	connectionObj.changes[pathHash].hasher = hasher
}

func processCommand(processorChan chan FileCommand, allocationObj *Allocation, connectionID, clientID, pathHash string) {

	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("Recovered panic", zap.String("connection_id", connectionID), zap.Any("error", r))
			SetError(connectionID, pathHash, common.NewError("panic", "Recovered panic"))
		}
	}()

	for cmd := range processorChan {
		if cmd == nil {
			return
		}
		logging.Logger.Info("Processing command", zap.String("connection_id", connectionID), zap.String("path", cmd.GetPath()))
		res, err := cmd.ProcessContent(allocationObj)
		if err != nil {
			logging.Logger.Error("Error processing command", zap.String("connection_id", connectionID), zap.String("path", cmd.GetPath()), zap.Error(err))
			SetError(connectionID, pathHash, err)
			return
		}
		err = cmd.ProcessThumbnail(allocationObj)
		if err != nil {
			logging.Logger.Error("Error processing command", zap.String("connection_id", connectionID), zap.String("path", cmd.GetPath()), zap.Error(err))
			SetError(connectionID, pathHash, err)
			return
		}
		if res.IsFinal {
			err = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
				connectionObj, err := GetAllocationChanges(ctx, connectionID, allocationObj.ID, clientID)
				if err != nil {
					return err
				}
				return cmd.UpdateChange(ctx, connectionObj)
			})
			if err != nil {
				logging.Logger.Error("Error processing command", zap.String("connection_id", connectionID), zap.String("path", cmd.GetPath()), zap.Error(err))
				SetError(connectionID, pathHash, err)
			}
			return
		}
	}

}

func drainChan(processorChan chan FileCommand) {
	for {
		select {
		case _, ok := <-processorChan:
			if !ok {
				return
			}
		default:
			return
		}
	}
}

// DeleteConnectionObjEntry remove the connectionID entry from map
// If the given connectionID is not present, then it is no-op.
func DeleteConnectionObjEntry(connectionID string) {
	connectionObjMutex.Lock()
	delete(connectionProcessor, connectionID)
	connectionObjMutex.Unlock()
}

// cleanConnectionObj cleans the connectionObjSize map. It deletes the rows
// for which deadline is exceeded.
func cleanConnectionObj() {
	connectionObjMutex.Lock()
	for connectionID, connectionObj := range connectionProcessor {
		diff := time.Since(connectionObj.UpdatedAt)
		if diff >= ConnectionObjTimeout {
			delete(connectionProcessor, connectionID)
		}
	}
	connectionObjMutex.Unlock()
}

func startCleanConnectionObj(ctx context.Context) {
	ticker := time.NewTicker(ConnectionObjCleanInterval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanConnectionObj()
		}

	}
}

func SetupWorkers(ctx context.Context) {
	go startCleanConnectionObj(ctx)
}
