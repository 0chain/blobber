package challenge

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go.uber.org/zap"

	"0chain.net/datastore"
	"0chain.net/filestore"
	"0chain.net/lock"
	. "0chain.net/logging"
)

var dataStore datastore.Store
var fileStore filestore.FileStore

func SetupWorkers(ctx context.Context, metaStore datastore.Store, fsStore filestore.FileStore) {
	dataStore = metaStore
	fileStore = fsStore
	go FindChallenges(ctx)
}

var challengeHandler = func(ctx context.Context, key datastore.Key, value []byte) error {
	challengeObj := Provider().(*ChallengeEntity)
	err := json.Unmarshal(value, challengeObj)
	if err != nil {
		return err
	}
	mutex := lock.GetMutex(challengeObj.GetKey())
	mutex.Lock()
	if challengeObj.Status != Committed && challengeObj.Status != Failed && numOfWorkers < 1 {
		Logger.Info("Starting challenge with ID: " + challengeObj.ID)
		if challengeObj.Status == Error {
			challengeObj.Retries++
		}
		numOfWorkers++
		challengeWorker.Add(1)
		go func() {
			ctx = dataStore.WithConnection(ctx)
			err := challengeObj.SendDataBlockToValidators(ctx, fileStore)
			if err != nil {
				Logger.Error("Error in responding to challenge. ", zap.Any("error", err.Error()))
			}
			err = dataStore.Commit(ctx)
			if err != nil {
				Logger.Error("Error in challenge commit to DB", zap.Error(err))
			}
			challengeWorker.Done()
			mutex.Unlock()
		}()
	} else {
		mutex.Unlock()
	}
	return nil
}

var challengeWorker sync.WaitGroup
var numOfWorkers = 0

func FindChallenges(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	for true {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if numOfWorkers == 0 {

				dataStore.IteratePrefix(ctx, "challenge:", challengeHandler)
				challengeWorker.Wait()
				numOfWorkers = 0
			}

		}
	}

}
