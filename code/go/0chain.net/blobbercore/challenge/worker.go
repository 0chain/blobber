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
	// toSubmitChannel    = make(chan *ChallengeEntity, 100)
	challengeMap     = treemap.NewWith(Int64Comparator)
	challengeMapLock = sync.RWMutex{}
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
			it.createChallenge()
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
		logging.Logger.Info("get_commit_batch")
		challenges := getBatch(batchSize)
		if len(challenges) == 0 {
			time.Sleep(2 * time.Second)
			continue
		}

		logging.Logger.Info("committing_challenge_tickets", zap.Any("num", len(challenges)), zap.Any("challenges", challenges))

		for _, challenge := range challenges {
			txn, _ := challenge.getCommitTransaction()
			if txn != nil {
				wg.Add(1)
				go func(challenge *ChallengeEntity) {
					defer func() {
						if r := recover(); r != nil {
							logging.Logger.Error("verifyChallengeTransaction", zap.Any("err", r))
						}
					}()
					logging.Logger.Info("submitting_challenge_start", zap.Any("challenge_id", challenge.ChallengeID))
					err := challenge.VerifyChallengeTransaction(txn)
					logging.Logger.Info("submitting_challenge_over", zap.Any("challenge_id", challenge.ChallengeID), zap.Any("err", err))
					if err == nil || err != ErrValNotPresent {
						deleteChallenge(int64(challenge.CreatedAt))
					}
					wg.Done()
				}(challenge)
			}
		}

		wg.Wait()
	}
}

func getBatch(batchSize int) (chall []*ChallengeEntity) {
	challengeMapLock.RLock()
	defer challengeMapLock.RUnlock()

	logging.Logger.Info("getBatch", zap.Any("size", challengeMap.Size()))

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

func (it *ChallengeEntity) createChallenge() {
	logging.Logger.Info("create_challenge", zap.Int64("created_at", int64(it.CreatedAt)))
	challengeMapLock.Lock()
	challengeMap.Put(int64(it.CreatedAt), it)
	challengeMapLock.Unlock()
}

func deleteChallenge(key int64) {
	challengeMapLock.Lock()
	challengeMap.Remove(key)
	challengeMapLock.Unlock()
}
