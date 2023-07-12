package handler

import (
	"fmt"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

type DownloadQuota struct {
	Quota int64
	sync.Mutex
}

type QuotaManager struct {
	m   map[string]*DownloadQuota
	mux sync.RWMutex
}

var quotaManagerInstance *QuotaManager
var quotaManagerOnce sync.Once

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
	qm.mux.RLock()
	defer qm.mux.RUnlock()
	return qm.m[key]
}

func (qm *QuotaManager) createOrUpdateQuota(numBlocks int64, key string) {
	qm.mux.Lock()
	defer qm.mux.Unlock()

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
