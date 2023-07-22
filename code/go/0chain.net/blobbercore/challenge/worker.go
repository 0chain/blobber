package challenge

import (
	"context"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/emirpasic/gods/maps/treemap"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
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

const batchSize = 5

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
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[processor]challenge", zap.Any("err", r))
		}
	}()
	numWorkers := config.Configuration.ChallengeResolveNumWorkers
	sem := semaphore.NewWeighted(int64(numWorkers))
	logging.Logger.Info("initializing challenge workers",
		zap.Int("num_workers", numWorkers))
	for {
		select {
		case <-ctx.Done():
			logging.Logger.Info("exiting challengeProcessor")
			return

		case it := <-toProcessChallenge:

			logging.Logger.Info("processing_challenge", zap.Any("challenge_id", it.ChallengeID))
			if ok := it.createChallenge(); !ok {
				continue
			}
			err := sem.Acquire(ctx, 1)
			if err != nil {
				logging.Logger.Error("failed to acquire semaphore", zap.Error(err))
				continue
			}
			go func(it *ChallengeEntity) {
				processChallenge(ctx, it)
				sem.Release(1)
			}(it)
		}
	}
}

func processChallenge(ctx context.Context, it *ChallengeEntity) {

	logging.Logger.Info("processing_challenge",
		zap.String("challenge_id", it.ChallengeID))

	validateOnValidators(it)
}

func commitOnChainWorker(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[commitWorker]challenge", zap.Any("err", r))
		}
	}()
	wg := sync.WaitGroup{}
	for {
		select {
		case <-ctx.Done():
			logging.Logger.Info("exiting commitOnChainWorker")
			return
		default:
		}
		// Batch size to commit on chain
		challenges := getBatch(batchSize)
		if len(challenges) == 0 {
			time.Sleep(2 * time.Second)
			continue
		}

		logging.Logger.Info("committing_challenge_tickets", zap.Any("num", len(challenges)), zap.Any("challenges", challenges))

		for _, challenge := range challenges {
			chall := challenge
			txn, _ := chall.getCommitTransaction()
			if txn != nil {
				wg.Add(1)
				go func(challenge *ChallengeEntity) {
					defer func() {
						wg.Done()
						if r := recover(); r != nil {
							logging.Logger.Error("verifyChallengeTransaction", zap.Any("err", r))
						}
					}()
					err := challenge.VerifyChallengeTransaction(txn)
					if err == nil || err != ErrEntityNotFound {
						deleteChallenge(int64(challenge.RoundCreatedAt))
					}
				}(&chall)
			}
		}
		wg.Wait()
	}
}

func getBatch(batchSize int) (chall []ChallengeEntity) {
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
		if ticket.Status != Processed {
			break
		}
		chall = append(chall, *ticket)
	}
	return
}

func (it *ChallengeEntity) createChallenge() bool {
	challengeMapLock.Lock()
	if _, ok := challengeMap.Get(it.RoundCreatedAt); ok {
		challengeMapLock.Unlock()
		return false
	}
	challengeMap.Put(it.RoundCreatedAt, it)
	challengeMapLock.Unlock()
	return true
}

func deleteChallenge(key int64) {
	challengeMapLock.Lock()
	challengeMap.Remove(key)
	challengeMapLock.Unlock()
}
