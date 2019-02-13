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
	if challengeObj.Status != Committed && challengeObj.Status != Failed && numOfWorkers < 1 && challengeObj.Retries < 10 {
		numOfWorkers++
		Logger.Info("Starting challenge with ID: " + challengeObj.ID)
		if challengeObj.Status == Error {
			challengeObj.Retries++
		}
		challengeWorker.Add(1)
		go func() {
			newctx := dataStore.WithConnection(ctx)
			err := challengeObj.SendDataBlockToValidators(newctx, fileStore)
			if err != nil {
				Logger.Error("Error in responding to challenge. ", zap.Any("error", err.Error()))
			}
			err = dataStore.Commit(newctx)
			if err != nil {
				Logger.Error("Error in challenge commit to DB", zap.Error(err))
			}
			challengeWorker.Done()
			mutex.Unlock()
			Logger.Info("Challenge has been processed", zap.Any("id", challengeObj.ID), zap.String("txn", challengeObj.CommitTxnID))
		}()
	} else {
		mutex.Unlock()
	}
	return nil
}

var challengeWorker sync.WaitGroup
var numOfWorkers = 0
var iterInprogress = false

func FindChallenges(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	for true {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !iterInprogress && numOfWorkers == 0 {
				iterInprogress = true
				dataStore.IteratePrefix(ctx, "challenge:", challengeHandler)
				challengeWorker.Wait()
				iterInprogress = false
				numOfWorkers = 0
			}
		}
	}

}
