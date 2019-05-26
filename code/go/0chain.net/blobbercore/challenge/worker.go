package challenge

import (
	
	"context"
	
	"sync"
	"time"

	"0chain.net/core/lock"
	."0chain.net/core/logging"
	"0chain.net/blobbercore/datastore"
	"0chain.net/blobbercore/config"

	"go.uber.org/zap"
)

type BCChallengeResponse struct {
	BlobberID  string             `json:"blobber_id"`
	Challenges []*ChallengeEntity `json:"challenges"`
}

func SetupWorkers(ctx context.Context) {
	go FindChallenges(ctx)
}

func RespondToChallenge(ctx context.Context, challengeObj *ChallengeEntity) {
	mutex := lock.GetMutex(challengeObj.TableName(), challengeObj.ChallengeID)
	mutex.Lock()
	newctx := datastore.GetStore().CreateTransaction(ctx)

	err := challengeObj.SendDataBlockToValidators(newctx)
	if err != nil {
		Logger.Error("Error in responding to challenge. ", zap.Any("error", err.Error()))
	}

	db:= datastore.GetStore().GetTransaction(newctx)
	err = db.Commit().Error

	if err != nil {
		Logger.Error("Error in challenge commit to DB", zap.Error(err), zap.String("challenge_id", challengeObj.ChallengeID))
	}

	mutex.Unlock()

	// if challengeObj.ObjectPath != nil && challengeObj.Status == Committed && challengeObj.ObjectPath.FileBlockNum > 0 {
	// 	//stats.FileChallenged(challengeObj.AllocationID, challengeObj.ObjectPath.Meta["path"].(string), challengeObj.CommitTxnID)
	// 	if challengeObj.Result == ChallengeSuccess {
	// 		go stats.AddChallengeRedeemedEvent(challengeObj.AllocationID, challengeObj.ID, stats.SUCCESS, stats.REDEEMSUCCESS, challengeObj.ObjectPath.Meta["path"].(string), challengeObj.CommitTxnID)
	// 	} else if challengeObj.Result == ChallengeFailure {
	// 		go stats.AddChallengeRedeemedEvent(challengeObj.AllocationID, challengeObj.ID, stats.FAILED, stats.REDEEMSUCCESS, challengeObj.ObjectPath.Meta["path"].(string), challengeObj.CommitTxnID)
	// 	}

	// } else if challengeObj.ObjectPath != nil && challengeObj.Status != Committed && challengeObj.ObjectPath.FileBlockNum > 0 && challengeObj.Retries >= config.Configuration.ChallengeMaxRetires {
	// 	if challengeObj.Result == ChallengeSuccess {
	// 		go stats.AddChallengeRedeemedEvent(challengeObj.AllocationID, challengeObj.ID, stats.SUCCESS, stats.REDEEMERROR, challengeObj.ObjectPath.Meta["path"].(string), challengeObj.CommitTxnID)
	// 	} else if challengeObj.Result == ChallengeFailure {
	// 		go stats.AddChallengeRedeemedEvent(challengeObj.AllocationID, challengeObj.ID, stats.FAILED, stats.REDEEMERROR, challengeObj.ObjectPath.Meta["path"].(string), challengeObj.CommitTxnID)
	// 	}
	// }
	newctx.Done()
	challengeWorker.Done()
	Logger.Info("Challenge has been processed", zap.Any("id", challengeObj.ChallengeID), zap.String("txn", challengeObj.CommitTxnID))
}

var challengeWorker sync.WaitGroup
var iterInprogress = false

func FindChallenges(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.Configuration.ChallengeResolveFreq) * time.Second)
	for true {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !iterInprogress {
			// 	unredeemedMarkers = list.New()
			// 	iterInprogress = true
			// 	rctx := dataStore.WithReadOnlyConnection(context.Background())
			// 	dataStore.IteratePrefix(rctx, "challenge:", challengeHandler)
			// 	dataStore.Discard(rctx)
			// 	rctx.Done()
			// 	for e := unredeemedMarkers.Front(); e != nil; e = e.Next() {
			// 		if numOfWorkers < config.Configuration.ChallengeResolveNumWorkers {
			// 			numOfWorkers++
			// 			challengeWorker.Add(1)
			// 			Logger.Info("Starting challenge with ID: " + e.Value.(string))
			// 			go RespondToChallenge(e.Value.(string))
			// 		} else {
			// 			challengeWorker.Wait()
			// 		}
			// 	}
			// 	if numOfWorkers > 0 {
			// 		challengeWorker.Wait()
			// 	}

			// 	iterInprogress = false
			// 	numOfWorkers = 0
			// 	params := make(map[string]string)
			// 	params["blobber"] = node.Self.ID
			// 	var blobberChallenges BCChallengeResponse
			// 	blobberChallenges.Challenges = make([]*ChallengeEntity, 0)

			// 	handler := func(responseMap map[string][]byte, numSharders int, err error) {
			// 		openChallengeMap := make(map[string]int)
			// 		for _, v := range responseMap {
			// 			var blobberChallengest BCChallengeResponse
			// 			blobberChallengest.Challenges = make([]*ChallengeEntity, 0)
			// 			bytesReader := bytes.NewBuffer(v)
			// 			d := json.NewDecoder(bytesReader)
			// 			d.UseNumber()
			// 			errd := d.Decode(&blobberChallengest)
			// 			if errd != nil {
			// 				Logger.Error("Error in unmarshal of the sharder response", zap.Error(errd))
			// 				continue
			// 			}
			// 			for _, challenge := range blobberChallengest.Challenges {
			// 				if _, ok := openChallengeMap[challenge.ID]; !ok {
			// 					openChallengeMap[challenge.ID] = 0
			// 				}
			// 				openChallengeMap[challenge.ID]++
			// 				if openChallengeMap[challenge.ID] > (numSharders / 2) {
			// 					blobberChallenges.Challenges = append(blobberChallenges.Challenges, challenge)
			// 				}
			// 			}
			// 		}
			// 	}

			// 	transaction.MakeSCRestAPICall(transaction.STORAGE_CONTRACT_ADDRESS, "/openchallenges", params, chain.GetServerChain(), handler)
			// 	tCtx := dataStore.WithConnection(ctx)
			// 	for _, v := range blobberChallenges.Challenges {
			// 		if v == nil || len(v.ID) == 0 {
			// 			Logger.Info("No challenge entity from the challenge map")
			// 			continue
			// 		}
			// 		challengeObj := v
			// 		err := challengeObj.Read(tCtx, v.GetKey())
			// 		if err == datastore.ErrKeyNotFound {
			// 			Logger.Info("Adding new challenge found from blockchain", zap.String("challenge", v.ID))
			// 			writeMarkerEntity := writemarker.Provider().(*writemarker.WriteMarkerEntity)
			// 			writeMarkerEntity.WM = &writemarker.WriteMarker{AllocationID: challengeObj.AllocationID, AllocationRoot: challengeObj.AllocationRoot}

			// 			err = writeMarkerEntity.Read(tCtx, writeMarkerEntity.GetKey())
			// 			if err != nil {
			// 				continue
			// 			}
			// 			challengeObj.WriteMarker = writeMarkerEntity.GetKey()
			// 			challengeObj.ValidationTickets = make([]*ValidationTicket, len(challengeObj.Validators))
			// 			challengeObj.Write(tCtx)
			// 			go stats.AddNewChallengeEvent(challengeObj.AllocationID, challengeObj.ID)
			// 		}
			// 	}
			// 	dataStore.Commit(tCtx)
			// 	tCtx.Done()
			}
		}
	}

}
