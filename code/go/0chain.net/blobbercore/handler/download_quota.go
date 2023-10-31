package handler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

type DownloadQuota struct {
	Quota int64
	sync.Mutex
}

type QuotaManager struct {
	m   map[string]*DownloadQuota
	mux sync.RWMutex
}

var (
	quotaManagerInstance *QuotaManager
	quotaManagerOnce     sync.Once
	downloadLimit        = make(map[string]int64)
	downloadLock         sync.RWMutex
)

func addDailyBlocks(key string, numBlocks int64) {
	downloadLock.Lock()
	defer downloadLock.Unlock()
	downloadLimit[key] += numBlocks
}

func getDailyBlocks(key string) int64 {
	downloadLock.RLock()
	defer downloadLock.RUnlock()
	return downloadLimit[key]
}

func startDownloadLimitCleanup(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(24 * time.Hour):
			downloadLock.Lock()
			downloadLimit = make(map[string]int64)
			downloadLock.Unlock()
		}
	}
}

func getQuotaManager() *QuotaManager {
	quotaManagerOnce.Do(func() {
		quotaManagerInstance = newQuotaManager()
	})
	return quotaManagerInstance
}

func newQuotaManager() *QuotaManager {
	return &QuotaManager{
		m: make(map[string]*DownloadQuota),
	}
}

func (qm *QuotaManager) getDownloadQuota(key string) *DownloadQuota {
	logging.Logger.Info("getDownloadQuota", zap.String("connectionID", key))
	qm.mux.RLock()
	defer qm.mux.RUnlock()
	return qm.m[key]
}

func (qm *QuotaManager) createOrUpdateQuota(numBlocks int64, key string) {
	qm.mux.Lock()
	defer qm.mux.Unlock()
	logging.Logger.Info("createOrUpdateQuota", zap.String("connectionID", key))
	if dq, ok := qm.m[key]; ok {
		dq.Lock()
		dq.Quota += numBlocks
		dq.Unlock()
	} else {
		qm.m[key] = &DownloadQuota{
			Quota: numBlocks,
		}
	}
}

func (qm *QuotaManager) consumeQuota(key string, numBlocks int64) error {
	qm.mux.Lock()
	defer qm.mux.Unlock()

	dq, ok := qm.m[key]
	if !ok {
		return common.NewError("consume_quota", "no download quota")
	}
	err := dq.consumeQuota(numBlocks)
	if err != nil {
		return err
	}
	if dq.Quota == 0 {
		delete(qm.m, key)
	}
	return nil
}

func (dq *DownloadQuota) consumeQuota(numBlocks int64) error {
	dq.Lock()
	defer dq.Unlock()
	if dq.Quota < numBlocks {
		return common.NewError("consume_quota", fmt.Sprintf("insufficient quota: available %v, requested %v", dq.Quota, numBlocks))
	}
	dq.Quota -= numBlocks
	return nil
}
