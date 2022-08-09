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
}

var nextValidateChallenge = make(chan TodoChallenge, config.Configuration.ChallengeResolveNumWorkers*100)
var nextCommitChallenge = make(chan TodoChallenge, config.Configuration.ChallengeResolveNumWorkers*100)

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
	for i := 0; i < config.Configuration.ChallengeResolveNumWorkers; i++ {
		go validateAccepted(ctx)
		go commitValidated(ctx)
	}

	// populate all accepted/processed challenges to channel
	loadTodoChallenges()
}

func validateAccepted(ctx context.Context) {

	for {
		select {
		case <-ctx.Done():
			logging.Logger.Info("exiting validateAccepted")
			return

		case it := <-nextValidateChallenge:

			logging.Logger.Info("validating_challenge",
				zap.String("challenge_id", it.Id))

			now := time.Now()
			if now.Sub(it.CreatedAt) > config.Configuration.ChallengeCompletionTime {
				c := &ChallengeEntity{ChallengeID: it.Id}
				c.CancelChallenge(ctx, ErrExpiredCCT)

				logging.Logger.Error("[challenge]timeout",
					zap.Any("challenge_id", it.Id),
					zap.String("status", Accepted.String()),
					zap.Time("created", it.CreatedAt),
					zap.Time("start", now),
					zap.String("delay", now.Sub(it.CreatedAt).String()),
					zap.String("cct", config.Configuration.ChallengeCompletionTime.String()))
				continue
			}

			logging.Logger.Info("[challenge]next:"+strings.ToLower(Accepted.String()),
				zap.Any("challenge_id", it.Id),
				zap.String("status", Accepted.String()),
				zap.Time("created", it.CreatedAt),
				zap.Time("start", now),
				zap.String("delay", now.Sub(it.CreatedAt).String()),
				zap.String("cct", config.Configuration.ChallengeCompletionTime.String()))

			validateOnValidators(it.Id)

		}
	}
}

func commitValidated(ctx context.Context) {

	for {
		select {
		case <-ctx.Done():
			logging.Logger.Info("exiting commitValidated")
			return

		case it := <-nextCommitChallenge:

			logging.Logger.Info("committing_challenge",
				zap.String("challenge_id", it.Id))

			now := time.Now()
			if now.Sub(it.CreatedAt) > config.Configuration.ChallengeCompletionTime {
				c := &ChallengeEntity{ChallengeID: it.Id}
				c.CancelChallenge(ctx, ErrExpiredCCT)

				logging.Logger.Error("[challenge]timeout",
					zap.Any("challenge_id", it.Id),
					zap.String("status", Processed.String()),
					zap.Time("created", it.CreatedAt),
					zap.Time("start", now),
					zap.String("delay", now.Sub(it.CreatedAt).String()),
					zap.String("cct", config.Configuration.ChallengeCompletionTime.String()))
				continue
			}

			logging.Logger.Info("[challenge]next:"+strings.ToLower(Processed.String()),
				zap.Any("challenge_id", it.Id),
				zap.String("status", Processed.String()),
				zap.Time("created", it.CreatedAt),
				zap.Time("start", now),
				zap.String("delay", now.Sub(it.CreatedAt).String()),
				zap.String("cct", config.Configuration.ChallengeCompletionTime.String()))

			commitOnChain(it.Id)

		}
	}
}
