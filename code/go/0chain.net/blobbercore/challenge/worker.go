package challenge

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
)

// SetupWorkers start challenge workers
func SetupWorkers(ctx context.Context) {
	go startSyncOpen(ctx)
	go startProcessAccepted(ctx)
	go startCommitProcessed(ctx)
}

func startCommitProcessed(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.Configuration.ChallengeResolveFreq) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			commitProcessed(ctx)
		}
	}
}

func startProcessAccepted(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.Configuration.ChallengeResolveFreq) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processAccepted(ctx)
		}
	}
}

// startSyncOpen
func startSyncOpen(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.Configuration.ChallengeResolveFreq) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			syncOpenChallenges(ctx)
		}
	}
}
