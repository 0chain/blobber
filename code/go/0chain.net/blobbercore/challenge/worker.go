package challenge

import (
	"context"
	"time"
)

type TodoChallenge struct {
	Id        string
	CreatedAt time.Time
	Status    ChallengeStatus
}

// SetupWorkers start challenge workers
func SetupWorkers(ctx context.Context) {
	go syncOpenChallenges(ctx)
	go ProcessChallenge(ctx)
	go ProcessChallengeTransactions(ctx)
}
