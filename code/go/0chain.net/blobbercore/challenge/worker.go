package challenge

import (
	"context"
	"strings"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

type TodoChallenge struct {
	Id        string
	CreatedAt time.Time
	Status    ChallengeStatus
}

var toProcessChallenge = make(chan TodoChallenge, config.Configuration.ChallengeResolveNumWorkers)

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
	loadTodoChallenges(true)

	// populate all accepted/processed challenges to channel
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(config.Configuration.ChallengeResolveFreq) * time.Second):
			loadTodoChallenges(false)
		}
	}
}

func challengeProcessor(ctx context.Context) {

	for {
		select {
		case <-ctx.Done():
			logging.Logger.Info("exiting challengeProcessor")
			return

		case it := <-toProcessChallenge:

			logging.Logger.Info("processing_challenge",
				zap.String("challenge_id", it.Id))

			now := time.Now()
			if now.Sub(it.CreatedAt) > config.StorageSCConfig.ChallengeCompletionTime {
				c := &ChallengeEntity{ChallengeID: it.Id}
				c.CancelChallenge(ctx, ErrExpiredCCT)

				logging.Logger.Error("[challenge]timeout",
					zap.Any("challenge_id", it.Id),
					zap.String("status", it.Status.String()),
					zap.Time("created", it.CreatedAt),
					zap.Time("start", now),
					zap.String("delay", now.Sub(it.CreatedAt).String()),
					zap.String("cct", config.StorageSCConfig.ChallengeCompletionTime.String()))
				continue
			}

			logging.Logger.Info("[challenge]next:"+strings.ToLower(it.Status.String()),
				zap.Any("challenge_id", it.Id),
				zap.String("status", it.Status.String()),
				zap.Time("created", it.CreatedAt),
				zap.Time("start", now),
				zap.String("delay", now.Sub(it.CreatedAt).String()),
				zap.String("cct", config.StorageSCConfig.ChallengeCompletionTime.String()))

			switch it.Status {
			case Accepted:
				validateOnValidators(it.Id)
			case Processed:
				commitOnChain(nil, it.Id)
			default:
				logging.Logger.Warn("[challenge]skipped",
					zap.Any("challenge_id", it.Id),
					zap.String("status", it.Status.String()),
					zap.Time("created", it.CreatedAt),
					zap.Time("start", now),
					zap.String("delay", now.Sub(it.CreatedAt).String()),
					zap.String("cct", config.StorageSCConfig.ChallengeCompletionTime.String()))
			}

		}
	}
}
