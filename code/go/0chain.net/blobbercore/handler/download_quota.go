package handler

import (
	"errors"
	"sync"
)

type DownloadQuota struct {
	Quota int64
	sync.Mutex
}

type QuotaManager struct {
	m sync.Map
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
	return &QuotaManager{}
}

func (qm *QuotaManager) getDownloadQuota(key string) *DownloadQuota {
	value, ok := qm.m.Load(key)
	if !ok {
		return nil
	}

	return value.(*DownloadQuota)
}

// TODO: Factor in excess quota
func (qm *QuotaManager) createOrUpdateQuota(numBlocks int64, key string) {
	dq, _ := qm.m.LoadOrStore(key, &DownloadQuota{})
	downloadQuota := dq.(*DownloadQuota)

	downloadQuota.Lock()
	downloadQuota.Quota += numBlocks
	downloadQuota.Unlock()
}

func (qm *QuotaManager) consumeQuota(key string, numBlocks int64) error {
	dq := qm.getDownloadQuota(key)
	if dq == nil {
		return errors.New("quota not found")
	}
	shouldDelete := dq.consumeQuota(numBlocks)
	if shouldDelete {
		qm.m.Delete(key)
	}
	return nil
}

func (dq *DownloadQuota) consumeQuota(numBlocks int64) bool {
	dq.Lock()
	defer dq.Unlock()
	if dq.Quota < numBlocks {
		return false
	}
	dq.Quota -= numBlocks
	return dq.Quota == 0
}
