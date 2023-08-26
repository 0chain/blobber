package allocation

import (
	"context"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
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
	BaseChanger  *BaseFileChanger
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
	}
	connectionProcessor[connectionID].changes[pathHash] = connChange
	connChange.wg.Add(1)
	go func() {
		processCommand(connChange.ProcessChan, connectionObj.AllocationObj, connectionID, connectionObj.ClientID)
		connChange.wg.Done()
	}()
	return connectionProcessor[connectionID].changes[pathHash]
}

func GetConnectionChange(connectionID, pathHash string) *ConnectionChange {
	connectionObjMutex.RLock()
	defer connectionObjMutex.RUnlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		return nil
	}
	return connectionProcessor[connectionID].changes[pathHash]
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
	return connectionObj.changes[pathHash].BaseChanger
}

func SaveFileChanger(connectionID string, fileChanger *BaseFileChanger) {
	connectionObjMutex.Lock()
	defer connectionObjMutex.Unlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		connectionProcessor[connectionID] = &ConnectionProcessor{
			UpdatedAt: time.Now(),
			changes:   make(map[string]*ConnectionChange),
		}
	}
	connectionProcessor[connectionID].changes[fileChanger.PathHash].BaseChanger = fileChanger
}

func SetFinalized(connectionID, pathHash string, cmd FileCommand) error {
	connectionObjMutex.Lock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		connectionObjMutex.Unlock()
		return common.NewError("connection_not_found", "connection not found")
	}
	connChange := connectionProcessor[connectionID].changes[pathHash]
	if connChange.isFinalized {
		connectionObjMutex.Unlock()
		return common.NewError("connection_change_finalized", "connection change finalized")
	}
	connChange.isFinalized = true
	connectionObjMutex.Unlock()
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
	connChange := connectionProcessor[connectionID].changes[pathHash]
	if connChange == nil {
		return common.NewError("connection_change_not_found", "connection change not found")
	}
	if connChange.isFinalized {
		return common.NewError("connection_change_finalized", "connection change finalized")
	}
	connectionObjMutex.RUnlock()
	connChange.ProcessChan <- cmd
	return nil
}

func GetConnectionProcessor(connectionID string) *ConnectionProcessor {
	connectionObjMutex.RLock()
	defer connectionObjMutex.RUnlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		return nil
	}
	return connectionProcessor[connectionID]
}

func CreateConnectionProcessor(connectionID string) *ConnectionProcessor {
	connectionObjMutex.Lock()
	defer connectionObjMutex.Unlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		connectionProcessor[connectionID] = &ConnectionProcessor{
			UpdatedAt: time.Now(),
			changes:   make(map[string]*ConnectionChange),
		}
	}
	return connectionProcessor[connectionID]
}

func SetError(connectionID, pathHash string, err error) {
	connectionObjMutex.Lock()
	defer connectionObjMutex.Unlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		return
	}
	connChange := connectionProcessor[connectionID].changes[pathHash]
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
	connChange := connectionProcessor[connectionID].changes[pathHash]
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
	return connectionProcessor[connectionID].Size
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
		connectionProcessor[connectionID] = &ConnectionProcessor{
			UpdatedAt: time.Now(),
			changes:   make(map[string]*ConnectionChange),
		}
	}
	connectionProcessor[connectionID].changes[pathHash] = &ConnectionChange{
		hasher: hasher,
	}
}

func processCommand(processorChan chan FileCommand, allocationObj *Allocation, connectionID, clientID string) {
	for {
		select {
		case cmd, ok := <-processorChan:
			if !ok || cmd == nil {
				return
			}
			res, err := cmd.ProcessContent(allocationObj)
			if err != nil {
				SetError(connectionID, cmd.GetPath(), err)
				return
			}
			err = cmd.ProcessThumbnail(allocationObj)
			if err != nil {
				SetError(connectionID, cmd.GetPath(), err)
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
					SetError(connectionID, cmd.GetPath(), err)
				}
				return
			}
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
