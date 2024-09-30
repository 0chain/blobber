package allocation

import (
	"context"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/seqpriorityqueue"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

var (
	// ConnectionObjCleanInterval start to clean the connectionObjMap
	ConnectionObjCleanInterval = 45 * time.Minute
	// ConnectionObjTimout after which connectionObj entry should be invalid
	ConnectionObjTimeout = 30 * time.Minute
)

var (
	connectionProcessor = make(map[string]*ConnectionProcessor)
	connectionObjMutex  sync.RWMutex
)

type ConnectionProcessor struct {
	Size         int64
	UpdatedAt    time.Time
	lock         sync.RWMutex
	changes      map[string]*ConnectionChange
	ClientID     string
	AllocationID string
	ctx          context.Context
	cnclCtx      context.CancelFunc
}

type ConnectionChange struct {
	hasher      *filestore.CommitHasher
	baseChanger *BaseFileChanger
	existingRef *reference.Ref
	isFinalized bool
	lock        sync.Mutex
	seqPQ       *seqpriorityqueue.SeqPriorityQueue
}

func GetFileChanger(connectionID, pathHash string) *BaseFileChanger {
	connectionObjMutex.RLock()
	connectionObj := connectionProcessor[connectionID]
	connectionObjMutex.RUnlock()
	if connectionObj == nil {
		return nil
	}
	connectionObj.lock.RLock()
	defer connectionObj.lock.RUnlock()
	if connectionObj.changes[pathHash] == nil {
		return nil
	}
	return connectionObj.changes[pathHash].baseChanger
}

func SaveFileChanger(connectionID string, fileChanger *BaseFileChanger) error {
	connectionObjMutex.RLock()
	connectionObj := connectionProcessor[connectionID]
	connectionObjMutex.RUnlock()
	if connectionObj == nil {
		return common.NewError("connection_not_found", "connection not found")
	}
	connectionObj.lock.Lock()
	if connectionObj.changes[fileChanger.LookupHash] == nil {
		return common.NewError("connection_change_not_found", "connection change not found")
	}
	connectionObj.changes[fileChanger.LookupHash].baseChanger = fileChanger
	connectionObj.lock.Unlock()
	return nil
}

func SaveExistingRef(connectionID, pathHash string, existingRef *reference.Ref) error {
	connectionObjMutex.RLock()
	connectionObj := connectionProcessor[connectionID]
	connectionObjMutex.RUnlock()
	if connectionObj == nil {
		return common.NewError("connection_not_found", "connection not found")
	}
	connectionObj.lock.Lock()
	defer connectionObj.lock.Unlock()
	if connectionObj.changes[pathHash] == nil {
		return common.NewError("connection_change_not_found", "connection change not found")
	}
	connectionObj.changes[pathHash].existingRef = existingRef
	return nil
}

func GetExistingRef(connectionID, pathHash string) *reference.Ref {
	connectionObjMutex.RLock()
	connectionObj := connectionProcessor[connectionID]
	connectionObjMutex.RUnlock()
	if connectionObj == nil {
		return nil
	}
	connectionObj.lock.RLock()
	defer connectionObj.lock.RUnlock()
	if connectionObj.changes[pathHash] == nil {
		return nil
	}
	return connectionObj.changes[pathHash].existingRef
}

func GetConnectionProcessor(connectionID string) *ConnectionProcessor {
	connectionObjMutex.RLock()
	defer connectionObjMutex.RUnlock()
	return connectionProcessor[connectionID]
}

func CreateConnectionProcessor(connectionID, allocationID, clientID string) *ConnectionProcessor {
	connectionObjMutex.Lock()
	defer connectionObjMutex.Unlock()
	connectionObj := connectionProcessor[connectionID]
	if connectionObj == nil {
		ctx, cnclCtx := context.WithCancel(context.Background())
		connectionObj = &ConnectionProcessor{
			UpdatedAt:    time.Now(),
			changes:      make(map[string]*ConnectionChange),
			AllocationID: allocationID,
			ClientID:     clientID,
			ctx:          ctx,
			cnclCtx:      cnclCtx,
		}
		connectionProcessor[connectionID] = connectionObj
	}
	return connectionObj
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
		ctx, cnclCtx := context.WithCancel(context.Background())
		connectionProcessor[connectionID] = &ConnectionProcessor{
			Size:      addSize,
			UpdatedAt: time.Now(),
			changes:   make(map[string]*ConnectionChange),
			ctx:       ctx,
			cnclCtx:   cnclCtx,
		}
		return
	}

	connectionObj.Size = connectionObj.Size + addSize
	logging.Logger.Info("UpdateConnectionObjSize", zap.String("connection_id", connectionID), zap.Int64("add_size", addSize), zap.Int64("size", connectionObj.Size))
	connectionObj.UpdatedAt = time.Now()
}

func SaveFileChange(ctx context.Context, connectionID, pathHash, fileName string, cmd FileCommand, isFinal bool, contentSize, offset, dataWritten, addSize int64) (bool, error) {
	connectionObjMutex.RLock()
	connectionObj := connectionProcessor[connectionID]
	connectionObjMutex.RUnlock()
	if connectionObj == nil {
		return false, common.NewError("connection_not_found", "connection not found for save file change")
	}
	connectionObj.lock.Lock()
	connectionObj.UpdatedAt = time.Now()
	saveChange := false
	change := connectionObj.changes[pathHash]
	if change == nil {
		change = &ConnectionChange{}
		connectionObj.changes[pathHash] = change
		change.lock.Lock()
		defer change.lock.Unlock()
		connectionObj.lock.Unlock()
		_, err := GetConnectionObj(ctx, connectionID, connectionObj.AllocationID, connectionObj.ClientID)
		if err != nil {
			return saveChange, err
		}
		err = cmd.AddChange(ctx)
		if err != nil {
			return saveChange, err
		}
		hasher := filestore.NewCommitHasher(contentSize)
		change.hasher = hasher
		change.seqPQ = seqpriorityqueue.NewSeqPriorityQueue(contentSize)
		go hasher.Start(connectionObj.ctx, connectionID, connectionObj.AllocationID, fileName, pathHash, change.seqPQ)
		saveChange = true
	} else {
		change.lock.Lock()
		defer change.lock.Unlock()
		connectionObj.lock.Unlock()
	}
	if change.isFinalized {
		return false, nil
	}

	if isFinal {
		change.isFinalized = true
		change.seqPQ.Done(seqpriorityqueue.UploadData{
			Offset:    offset,
			DataBytes: dataWritten,
		}, contentSize)
		if addSize != 0 {
			//check if reference exists and get the size
			existingSize, err := reference.GetObjectSizeByLookupHash(ctx, pathHash)
			if err != nil {
				return saveChange, err
			}
			addSize -= existingSize
			UpdateConnectionObjSize(connectionID, addSize)
		}
	} else {
		change.seqPQ.Push(seqpriorityqueue.UploadData{
			Offset:    offset,
			DataBytes: dataWritten,
		})
	}
	return saveChange, nil
}

func GetHasher(connectionID, pathHash string) *filestore.CommitHasher {
	connectionObjMutex.RLock()
	connectionObj := connectionProcessor[connectionID]
	connectionObjMutex.RUnlock()
	if connectionObj == nil {
		return nil
	}
	connectionObj.lock.RLock()
	defer connectionObj.lock.RUnlock()
	if connectionObj.changes[pathHash] == nil {
		return nil
	}
	return connectionObj.changes[pathHash].hasher
}

// DeleteConnectionObjEntry remove the connectionID entry from map
// If the given connectionID is not present, then it is no-op.
func DeleteConnectionObjEntry(connectionID string) {
	logging.Logger.Info("DeleteConnectionObjEntry", zap.String("connection_id", connectionID))
	connectionObjMutex.Lock()
	connectionObj, ok := connectionProcessor[connectionID]
	if ok {
		connectionObj.cnclCtx()
	}
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
			logging.Logger.Info("cleanConnectionObj", zap.String("connection_id", connectionID), zap.Duration("diff", diff))
			// Stop the context and hash worker
			connectionObj.cnclCtx()
			for _, change := range connectionObj.changes {
				if change.seqPQ != nil {
					change.seqPQ.Done(seqpriorityqueue.UploadData{}, 1)
				}
			}
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
