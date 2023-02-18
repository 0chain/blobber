package handler

import (
	"context"
	"time"

	blobConfig "github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/handler"
	valConfig "github.com/0chain/blobber/code/go/0chain.net/validatorcore/config"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

func StartHealthCheck(ctx context.Context, provider common.ProviderType) {

	var t time.Duration

	switch provider {
	case common.ProviderTypeBlobber:
		t = blobConfig.Configuration.HealthCheckWorkerFreq
	case common.ProviderTypeValidator:
		t = valConfig.Configuration.HealthCheckWorkerFreq

	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(t):
			go func() {
				start := time.Now()

				txnHash, err := handler.SendHealthCheck(provider)
				end := time.Now()
				if err == nil {
					logging.Logger.Info("success to send heartbeat", zap.String("txn_hash", txnHash), zap.Time("start", start), zap.Time("end", end), zap.Duration("duration", end.Sub(start)))
				} else {
					logging.Logger.Warn("failed to send heartbeat", zap.String("txn_hash", txnHash), zap.Time("start", start), zap.Time("end", end), zap.Duration("duration", end.Sub(start)))
				}
			}()
		}
	}
}
