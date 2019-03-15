package challenge

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"0chain.net/chain"
	"0chain.net/config"
	"0chain.net/datastore"
	"0chain.net/filestore"
	"0chain.net/lock"
	. "0chain.net/logging"

	"0chain.net/node"
	"0chain.net/transaction"
	"0chain.net/writemarker"
	"go.uber.org/zap"
)

type BCChallengeResponse struct {
	BlobberID    string                      `json:"blobber_id"`
	ChallengeMap map[string]*ChallengeEntity `json:"challenges"`
}

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
	if challengeObj.Status != Committed && challengeObj.Status != Failed && numOfWorkers < config.Configuration.ChallengeResolveNumWorkers && challengeObj.Retries < 10 {
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
	ticker := time.NewTicker(time.Duration(config.Configuration.ChallengeResolveFreq) * time.Second)
	for true {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			params := make(map[string]string)
			params["blobber"] = node.Self.ID
			var blobberChallenges BCChallengeResponse
			_, err := transaction.MakeSCRestAPICall(transaction.STORAGE_CONTRACT_ADDRESS, "/openchallenges", params, chain.GetServerChain(), &blobberChallenges)
			if err == nil {
				tCtx := dataStore.WithConnection(ctx)
				for k, v := range blobberChallenges.ChallengeMap {
					if v == nil {
						Logger.Info("No challenge entity from the challenge map")
						continue
					}
					challengeObj := v
					err = challengeObj.Read(tCtx, v.GetKey())
					if err == datastore.ErrKeyNotFound {
						Logger.Info("Adding new challenge found from blockchain", zap.String("challenge", k))
						writeMarkerEntity := writemarker.Provider().(*writemarker.WriteMarkerEntity)
						writeMarkerEntity.WM = &writemarker.WriteMarker{AllocationID: challengeObj.AllocationID, AllocationRoot: challengeObj.AllocationRoot}

						err = writeMarkerEntity.Read(tCtx, writeMarkerEntity.GetKey())
						if err != nil {
							continue
						}
						challengeObj.WriteMarker = writeMarkerEntity.GetKey()
						challengeObj.ValidationTickets = make([]*ValidationTicket, len(challengeObj.Validators))
						challengeObj.Write(tCtx)
					}
				}
				dataStore.Commit(tCtx)
			}

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
