package challenge

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

// SetupWorkers start challenge workers
func SetupWorkers(ctx context.Context) {
	go startSyncOpen(ctx)
	go startProcessAccepted(ctx)
	go startCommitProcessed(ctx)
}

func startCommitProcessed(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(config.Configuration.ChallengeResolveFreq) * time.Second):
			commitProcessed(ctx)
		}
	}
}

func startProcessAccepted(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(config.Configuration.ChallengeResolveFreq) * time.Second):
			processAccepted(ctx)
		}
	}
}

// startSyncOpen
func startSyncOpen(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(config.Configuration.ChallengeResolveFreq) * time.Second):
			logging.Logger.Info("[challenge]wait", zap.Int("count", cMap.Count()))
			syncOpenChallenges(ctx)
		}
	}
}
