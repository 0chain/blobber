package challenge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

type TodoChallenge struct {
	Id        string
	CreatedAt time.Time
	Status    ChallengeStatus
}

var nextTodoChallenge = make(chan TodoChallenge, config.Configuration.ChallengeResolveNumWorkers*100)

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

	// populate all accepted/processed challenges to channel
	loadTodoChallenges()

	// start challenge listeners
	for i := 0; i < config.Configuration.ChallengeResolveNumWorkers; i++ {
		go waitNextTodo(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			return
		}
	}

}

func waitNextTodo(ctx context.Context) {

	for {
		select {
		case <-ctx.Done():
			return

		case it := <-nextTodoChallenge:

			now := time.Now()
			if now.Sub(it.CreatedAt) > config.Configuration.ChallengeCompletionTime {
				db := datastore.GetStore().GetDB()
				db.Model(&ChallengeEntity{}).
					Where("challenge_id =? and status =? ", it.Id, it.Status).
					Updates(map[string]interface{}{
						"status":         Cancelled,
						"result":         ChallengeFailure,
						"status_message": fmt.Sprintf("status: %s,  created: %s, start: %s , delay: %s, cct: %s", it.Status, it.CreatedAt, now, now.Sub(it.CreatedAt).String(), config.Configuration.ChallengeCompletionTime.String()),
					})

				logging.Logger.Error("[challenge]timeout",
					zap.Any("challenge_id", it.Id),
					zap.String("status", it.Status.String()),
					zap.Time("created", it.CreatedAt),
					zap.Time("start", now),
					zap.String("delay", now.Sub(it.CreatedAt).String()),
					zap.String("cct", config.Configuration.ChallengeCompletionTime.String()))
				continue
			}

			logging.Logger.Info("[challenge]next:"+strings.ToLower(it.Status.String()),
				zap.Any("challenge_id", it.Id),
				zap.String("status", it.Status.String()),
				zap.Time("created", it.CreatedAt),
				zap.Time("start", now),
				zap.String("delay", now.Sub(it.CreatedAt).String()),
				zap.String("cct", config.Configuration.ChallengeCompletionTime.String()))

			switch it.Status {
			case Accepted:
				validateOnValidators(it.Id)
			case Processed:
				commitOnChain(it.Id)
			default:
				logging.Logger.Warn("[challenge]skipped",
					zap.Any("challenge_id", it.Id),
					zap.String("status", it.Status.String()),
					zap.Time("created", it.CreatedAt),
					zap.Time("start", now),
					zap.String("delay", now.Sub(it.CreatedAt).String()),
					zap.String("cct", config.Configuration.ChallengeCompletionTime.String()))
			}
		}
	}
}
