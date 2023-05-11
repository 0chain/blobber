package challenge

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"github.com/emirpasic/gods/maps/treemap"
	"github.com/remeh/sizedwaitgroup"
	"go.uber.org/zap"
)

type TodoChallenge struct {
	Id        string
	CreatedAt time.Time
	Status    ChallengeStatus
}

func Int64Comparator(a, b interface{}) int {
	aAsserted := a.(int64)
	bAsserted := b.(int64)
	switch {
	case aAsserted > bAsserted:
		return 1
	case aAsserted < bAsserted:
		return -1
	default:
		return 0
	}
}

var (
	toProcessChallenge = make(chan *ChallengeEntity, 100)
	challengeMap       = treemap.NewWith(Int64Comparator)
	challengeMapLock   = sync.RWMutex{}
)

// SetupWorkers start challenge workers
func SetupWorkers(ctx context.Context) {
	go startPullWorker(ctx)
	go startWorkers(ctx)
}

func startPullWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(config.Configuration.ChallengeResolveFreq) * time.Second):
			syncOpenChallenges(ctx)
		}
	}

}

func startWorkers(ctx context.Context) {
	// start challenge listeners
	go challengeProcessor(ctx)

	go commitOnChainWorker(ctx)
}

func challengeProcessor(ctx context.Context) {
	numWorkers := config.Configuration.ChallengeResolveNumWorkers
	swg := sizedwaitgroup.New(numWorkers)
	logging.Logger.Info("initializing challenge workers",
		zap.Int("num_workers", numWorkers))
	for {
		select {
		case <-ctx.Done():
			logging.Logger.Info("exiting challengeProcessor")
			return

		case it := <-toProcessChallenge:
			swg.Add()
			go func(it *ChallengeEntity) {
				processChallenge(ctx, it)
				swg.Done()
			}(it)

			swg.Wait()

			// switch it.Status {
			// case Accepted:
			// 	validateOnValidators(it.ChallengeID)
			// case Processed:
			// 	commitOnChain(nil, it.ChallengeID)
			// default:
			// 	logging.Logger.Warn("[challenge]skipped",
			// 		zap.Any("challenge_id", it.ChallengeID),
			// 		zap.String("status", it.Status.String()),
			// 		zap.Time("created", createdAt),
			// 		zap.Time("start", now),
			// 		zap.String("delay", now.Sub(createdAt).String()),
			// 		zap.String("cct", config.StorageSCConfig.ChallengeCompletionTime.String()))
			// }
		}
	}
}

func processChallenge(ctx context.Context, it *ChallengeEntity) {
	if exist := isExist(it.BlockNum); exist {
		// already processed
		return
	}

	logging.Logger.Info("processing_challenge",
		zap.String("challenge_id", it.ChallengeID))

	now := time.Now()
	createdAt := common.ToTime(it.CreatedAt)
	if now.Sub(createdAt) > config.StorageSCConfig.ChallengeCompletionTime {
		c := &ChallengeEntity{ChallengeID: it.ChallengeID}
		c.CancelChallenge(ctx, ErrExpiredCCT)

		logging.Logger.Error("[challenge]timeout",
			zap.Any("challenge_id", it.ChallengeID),
			zap.String("status", it.Status.String()),
			zap.Time("created", createdAt),
			zap.Time("start", now),
			zap.String("delay", now.Sub(createdAt).String()),
			zap.String("cct", config.StorageSCConfig.ChallengeCompletionTime.String()))
		return
	}
	createChallenge(it)
	if err := CreateChallengeTiming(it.ChallengeID, it.CreatedAt); err != nil {
		logging.Logger.Error("[challengetiming]add: ",
			zap.String("challenge_id", it.ChallengeID),
			zap.Time("created", createdAt),
			zap.Error(err))
	}
	logging.Logger.Info("[challenge]next:"+strings.ToLower(it.Status.String()),
		zap.Any("challenge_id", it.ChallengeID),
		zap.String("status", it.Status.String()),
		zap.Time("created", createdAt),
		zap.Time("start", now),
		zap.String("delay", now.Sub(createdAt).String()),
		zap.String("cct", config.StorageSCConfig.ChallengeCompletionTime.String()))

	validateOnValidators(it)
}

func commitOnChainWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// Batch size to commit on chain
		batchSize := 5

		challenges := getBatch(batchSize)
		if len(challenges) == 0 {
			time.Sleep(2 * time.Second)
			continue
		}

		type challengeResult struct {
			txn      *transaction.Transaction
			blockNum int64
			err      error
		}

		resp := make([]challengeResult, len(challenges))

		swg := sizedwaitgroup.New(5)
		failedIndex := len(challenges)
		mut := sync.Mutex{}
		//TODO: need to send them sequentially
		for index, challenge := range challenges {
			swg.Add()
			go func(challenge *ChallengeEntity, index int) {
				txn, err := commitOnChain(challenge, challenge.ChallengeID)
				if err != nil && txn != nil {
					mut.Lock()
					if index < failedIndex {
						failedIndex = index
					}
					mut.Unlock()
				}
				resp[index] = challengeResult{
					txn:      txn,
					blockNum: challenge.BlockNum,
					err:      err,
				}
				swg.Done()
			}(challenge, index)
		}

		for i := 0; i < failedIndex; i++ {
			deleteChallenge(resp[i].blockNum)
		}

		swg.Wait()
	}
}

func getBatch(batchSize int) (chall []*ChallengeEntity) {
	challengeMapLock.RLock()
	defer challengeMapLock.RUnlock()

	if challengeMap.Size() == 0 {
		return
	}

	it := challengeMap.Iterator()
	for it.Next() {
		if len(chall) >= batchSize {
			break
		}
		ticket := it.Value().(*ChallengeEntity)
		if ticket.Status != Processed && len(ticket.ValidationTickets) == 0 {
			break
		}
		chall = append(chall, ticket)
	}
	return
}

func isExist(key int64) bool {
	challengeMapLock.RLock()
	defer challengeMapLock.RUnlock()

	_, ok := challengeMap.Get(key)
	return ok
}

func createChallenge(it *ChallengeEntity) {
	challengeMapLock.Lock()
	challengeMap.Put(it.BlockNum, it)
	challengeMapLock.Unlock()
}

func getChallenge(key int64) *ChallengeEntity {
	challengeMapLock.RLock()
	defer challengeMapLock.RUnlock()

	val, ok := challengeMap.Get(key)
	if !ok {
		return nil
	}
	return val.(*ChallengeEntity)
}

func deleteChallenge(key int64) {
	challengeMapLock.Lock()
	challengeMap.Remove(key)
	challengeMapLock.Unlock()
}
