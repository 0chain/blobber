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
	go syncOpenChallenges_New(ctx)
	go ProcessChallenge_New(ctx)
}
