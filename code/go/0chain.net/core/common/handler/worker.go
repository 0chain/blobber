package handler

import (
	"context"
	"math"
	"time"

	blobConfig "github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/handler"
	valConfig "github.com/0chain/blobber/code/go/0chain.net/validatorcore/config"
	"github.com/0chain/gosdk/zcncore"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

const HARDFORK_NAME = "apollo"

var HardForkRound int64 = math.MaxInt64

func StartHealthCheck(ctx context.Context, provider common.ProviderType) {
	go setHardForkRound(ctx)
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
					logging.Logger.Warn("failed to send heartbeat", zap.String("txn_hash", txnHash), zap.Time("start", start), zap.Time("end", end), zap.Duration("duration", end.Sub(start)), zap.Error(err))
				}
			}()
		}
	}
}

func setHardForkRound(ctx context.Context) {
	HardForkRound, _ = zcncore.GetHardForkRound(HARDFORK_NAME)
	if HardForkRound == math.MaxInt64 {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Minute):
				HardForkRound, _ = zcncore.GetHardForkRound(HARDFORK_NAME)
				if HardForkRound != math.MaxInt64 {
					return
				}
			}
		}
	}
}
