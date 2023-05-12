package challenge

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/emirpasic/gods/maps/treemap"
	"github.com/remeh/sizedwaitgroup"
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
	if exist := isProcessed(int64(it.CreatedAt), it.ChallengeID); exist {
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
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[commitWorker]challenge", zap.Any("err", r))
		}
	}()
	swg := sizedwaitgroup.New(5)
	for {
		select {
		case <-ctx.Done():
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
			txn, _ := challenge.getCommitTransaction()
			if txn != nil {
				swg.Add()
				go func(challenge *ChallengeEntity) {
					defer func() {
						if r := recover(); r != nil {
							logging.Logger.Error("verifyTransactionWorker", zap.Any("err", r))
						}
					}()
					err := challenge.VerifyChallengeTransaction(ctx, txn)
					if err == nil || err != ErrValNotPresent {
						deleteChallenge(int64(challenge.CreatedAt))
					}
					swg.Done()
				}(challenge)
			}
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
		if ticket == nil {
			logging.Logger.Error("ticket is nil")
			continue
		}
		if ticket.Status != Processed && len(ticket.ValidationTickets) == 0 {
			break
		}
		chall = append(chall, ticket)
	}
	return
}

func isProcessed(key int64, id string) bool {
	challengeMapLock.RLock()
	_, ok := challengeMap.Get(key)
	challengeMapLock.RUnlock()
	if ok {
		return ok
	}
	db := datastore.GetStore().GetDB()
	var count int64
	db.Model(&ChallengeEntity{}).Where("challenge_id=?", id).Count(&count)
	return count > 0
}

func createChallenge(it *ChallengeEntity) {
	if it == nil {
		logging.Logger.Error("create_nil_challenge")
		return
	}
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
