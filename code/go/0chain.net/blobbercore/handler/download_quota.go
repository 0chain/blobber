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

func (qm *QuotaManager) createOrUpdateQuota(numBlocks int64, key string) {
	dqInterface, loaded := qm.m.Load(key)
	if !loaded {
		dq := &DownloadQuota{
			Quota: numBlocks,
		}
		qm.m.Store(key, dq)
		return
	}
	downloadQuota := dqInterface.(*DownloadQuota)
	downloadQuota.Lock()
	downloadQuota.Quota += numBlocks
	downloadQuota.Unlock()
}

func (qm *QuotaManager) consumeQuota(key string, numBlocks int64) error {
	dqInterface, ok := qm.m.Load(key)
	if !ok {
		return common.NewError("consume_quota", "no download quota")
	}
	dq := dqInterface.(*DownloadQuota)
	err := dq.consumeQuota(numBlocks)
	if err != nil {
		return err
	}
	dq.Lock()
	if dq.Quota == 0 {
		qm.m.Delete(key)
	}
	dq.Unlock()
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
