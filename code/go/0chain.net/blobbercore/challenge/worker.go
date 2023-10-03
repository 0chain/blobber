package challenge

import (
	"context"
	"github.com/0chain/gosdk/zboxcore/sdk"
	"github.com/0chain/gosdk/zcncore"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"github.com/emirpasic/gods/maps/treemap"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
)

const GetRoundInterval = 3 * time.Minute

type TodoChallenge struct {
	Id        string
	CreatedAt time.Time
	Status    ChallengeStatus
}

type RoundInfo struct {
	CurrentRound            int64
	CurrentRoundCaptureTime time.Time
	LastRoundDiff           int64
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
	roundInfo          = RoundInfo{}
)

const batchSize = 10

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
	go getRoundWorker(ctx)

	// start challenge listeners
	go challengeProcessor(ctx)

	go commitOnChainWorker(ctx)
}

func getRoundWorker(ctx context.Context) {
	ticker := time.NewTicker(GetRoundInterval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			network := zcncore.GetNetwork()
			currentRound, _ := sdk.GetRoundFromSharders(network.Sharders)

			if roundInfo.LastRoundDiff == 0 {
				roundInfo.LastRoundDiff = 1000
			} else {
				roundInfo.LastRoundDiff = currentRound - roundInfo.CurrentRound
			}
			roundInfo.CurrentRound = currentRound
			roundInfo.CurrentRoundCaptureTime = time.Now()
		}
	}
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
			var result bool
			_ = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
				result = it.createChallenge(ctx)
				return nil
			})
			if !result {
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

	_ = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		return validateOnValidators(ctx, it)
	})
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
			var (
				txn *transaction.Transaction
				err error
			)
			_ = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
				txn, err = chall.getCommitTransaction(ctx)
				return err
			})

			if txn != nil {
				wg.Add(1)
				go func(challenge *ChallengeEntity) {
					defer func() {
						wg.Done()
						if r := recover(); r != nil {
							logging.Logger.Error("verifyChallengeTransaction", zap.Any("err", r))
						}
					}()
					_ = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
						err := challenge.VerifyChallengeTransaction(ctx, txn)
						if err == nil || err != ErrEntityNotFound {
							deleteChallenge(challenge.RoundCreatedAt)
						}
						return nil
					})
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

	// create a set of string
	allocations := make(map[string]bool)

	it := challengeMap.Iterator()
	for it.Next() {
		if len(chall) >= batchSize {
			break
		}

		ticket := it.Value().(*ChallengeEntity)

		_, ok := allocations[ticket.AllocationID]
		if ticket.Status != Processed || ok {
			continue
		}
		chall = append(chall, *ticket)
		allocations[ticket.AllocationID] = true
	}
	return
}

func (it *ChallengeEntity) createChallenge(ctx context.Context) bool {
	db := datastore.GetStore().GetTransaction(ctx)

	challengeMapLock.Lock()
	defer challengeMapLock.Unlock()
	if _, ok := challengeMap.Get(it.RoundCreatedAt); ok {
		return false
	}
	var Found bool
	err := db.Raw("SELECT EXISTS(SELECT 1 FROM challenge_timing WHERE challenge_id = ?) AS found", it.ChallengeID).Scan(&Found).Error
	if err != nil {
		logging.Logger.Error("createChallenge", zap.Error(err))
		return false
	} else if Found {
		logging.Logger.Info("createChallenge", zap.String("challenge_id", it.ChallengeID), zap.String("status", "already exists"))
		return false
	}
	challengeMap.Put(it.RoundCreatedAt, it)
	return true
}

func deleteChallenge(key int64) {
	challengeMapLock.Lock()
	challengeMap.Remove(key)
	challengeMapLock.Unlock()
}
