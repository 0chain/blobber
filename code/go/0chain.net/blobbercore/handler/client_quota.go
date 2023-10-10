package handler

import (
	"context"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

var (
	clientMap    = make(map[string]*ClientStats)
	mpLock       sync.RWMutex
	blackListMap = make(map[string]bool)
	blMap        sync.RWMutex
)

const (
	BlackListWorkerTime = 6 * time.Hour
	Period              = 60 * 60 * 24 * 30 // 30 days
)

type ClientStats struct {
	ClientID      string           `gorm:"column:client_id;size:64;primaryKey" json:"client_id"`
	CreatedAt     common.Timestamp `gorm:"created_at;primaryKey" json:"created"`
	TotalUpload   uint64           `gorm:"column:total_upload" json:"total_upload"`
	TotalDownload uint64           `gorm:"column:total_download" json:"total_download"`
	TotalWM       uint64           `gorm:"column:total_write_marker" json:"total_write_marker"`
}

func (ClientStats) TableName() string {
	return "client_stats"
}

func (cs *ClientStats) BeforeCreate() error {
	cs.CreatedAt = common.Now()
	return nil
}

func GetUploadedData(clientID string) uint64 {
	mpLock.RLock()
	defer mpLock.RUnlock()
	cs := clientMap[clientID]
	if cs == nil {
		return 0
	}
	return cs.TotalUpload
}

func AddUploadedData(clientID string, data int64) {
	mpLock.Lock()
	defer mpLock.Unlock()
	cs := clientMap[clientID]
	if cs == nil {
		cs = &ClientStats{ClientID: clientID}
		clientMap[clientID] = cs
	}
	cs.TotalUpload += uint64(data)
}

func GetDownloadedData(clientID string) uint64 {
	mpLock.RLock()
	defer mpLock.RUnlock()
	cs := clientMap[clientID]
	return cs.TotalDownload
}

func AddDownloadedData(clientID string, data int64) {
	mpLock.Lock()
	defer mpLock.Unlock()
	cs := clientMap[clientID]
	if cs == nil {
		cs = &ClientStats{ClientID: clientID}
		clientMap[clientID] = cs
	}
	cs.TotalDownload += uint64(data)
}

func GetWriteMarkerCount(clientID string) uint64 {
	mpLock.RLock()
	defer mpLock.RUnlock()
	cs := clientMap[clientID]
	if cs == nil {
		return 0
	}
	return cs.TotalWM
}

func AddWriteMarkerCount(clientID string, count int64) {
	mpLock.Lock()
	defer mpLock.Unlock()
	cs := clientMap[clientID]
	if cs == nil {
		cs = &ClientStats{ClientID: clientID}
		clientMap[clientID] = cs
	}
	cs.TotalWM += uint64(count)
}

func CheckBlacklist(clientID string) bool {
	blMap.RLock()
	defer blMap.RUnlock()
	_, ok := blackListMap[clientID]
	return ok
}

func saveClientStats() {
	dbStats := make([]*ClientStats, 0, len(clientMap))
	mpLock.Lock()
	for _, cs := range clientMap {
		dbStats = append(dbStats, cs)
		delete(clientMap, cs.ClientID)
	}
	mpLock.Unlock()
	_ = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		tx := datastore.GetStore().GetTransaction(ctx)
		tx.Create(&dbStats)
		return nil
	})
	var blackList []string
	err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		tx := datastore.GetStore().GetTransaction(ctx)
		err := tx.Raw("SELECT client_id as blackList from (SELECT client_id,sum(total_upload) as upload,sum(total_download) as download, sum(total_write_marker) as writemarker from client_stats where created_at > ? group by client_id) as stats where stats.upload > ? or stats.download > ? or stats.writemarker > ?", common.Now()-Period, config.Configuration.UploadLimitMonthly, config.Configuration.BlockLimitMonthly, config.Configuration.CommitLimitMonthly).Scan(&blackList).Error
		return err
	})
	if err == nil {
		blMap.Lock()
		blackListMap = make(map[string]bool)
		for _, clientID := range blackList {
			blackListMap[clientID] = true
		}
		blMap.Unlock()
	}
}

func startBlackListWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(BlackListWorkerTime):
			saveClientStats()
		}
	}
}
