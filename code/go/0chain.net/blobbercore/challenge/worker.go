package challenge

import (
	"context"
	"strings"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

type TodoChallenge struct {
	Id        string
	CreatedAt time.Time
	Status    ChallengeStatus
}

// var toProcessChallenge = make(chan TodoChallenge, config.Configuration.ChallengeResolveNumWorkers)
var challengeEntityChan = make(chan *ChallengeEntity, config.Configuration.ChallengeResolveNumWorkers)

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

	numWorkers := config.Configuration.ChallengeResolveNumWorkers
	logging.Logger.Info("initializing challenge workers",
		zap.Int("num_workers", numWorkers))

	// start challenge listeners
	for i := 0; i < numWorkers; i++ {
		go challengeProcessor(ctx)
	}
	// to be run 1 time on init
	// loadTodoChallenges(true)

	// // populate all accepted/processed challenges to channel
	// for {
	// 	select {
	// 	case <-ctx.Done():
	// 		return
	// 	case <-time.After(time.Duration(config.Configuration.ChallengeResolveFreq) * time.Second):
	// 		loadTodoChallenges(false)
	// 	}
	// }
}

func challengeProcessor(ctx context.Context) {

	for {
		select {
		case <-ctx.Done():
			logging.Logger.Info("exiting challengeProcessor")
			return

		case it := <-challengeEntityChan:

			logging.Logger.Info("processing_challenge",
				zap.String("challenge_id", it.ChallengeID))

			now := time.Now()
			if now.Sub(common.ToTime(it.CreatedAt)) > config.StorageSCConfig.ChallengeCompletionTime {
				it.CancelChallenge(ctx, ErrExpiredCCT)

				logging.Logger.Error("[challenge]timeout",
					zap.Any("challenge_id", it.ChallengeID),
					zap.String("status", it.Status.String()),
					zap.Time("created", common.ToTime(it.CreatedAt)),
					zap.Time("start", now),
					zap.String("delay", now.Sub(common.ToTime(it.CreatedAt)).String()),
					zap.String("cct", config.StorageSCConfig.ChallengeCompletionTime.String()))
				continue
			}

			logging.Logger.Info("[challenge]next:"+strings.ToLower(it.Status.String()),
				zap.Any("challenge_id", it.ChallengeID),
				zap.String("status", it.Status.String()),
				zap.Time("created", common.ToTime(it.CreatedAt)),
				zap.Time("start", now),
				zap.String("delay", now.Sub(common.ToTime(it.CreatedAt)).String()),
				zap.String("cct", config.StorageSCConfig.ChallengeCompletionTime.String()))

			switch it.Status {
			case Accepted:
				validateOnValidators(it)
			case Processed:
				commitOnChain(it)
			default:
				logging.Logger.Warn("[challenge]skipped",
					zap.Any("challenge_id", it.ChallengeID),
					zap.String("status", it.Status.String()),
					zap.Time("created", common.ToTime(it.CreatedAt)),
					zap.Time("start", now),
					zap.String("delay", now.Sub(common.ToTime(it.CreatedAt)).String()),
					zap.String("cct", config.StorageSCConfig.ChallengeCompletionTime.String()))
			}

		}
	}
}
