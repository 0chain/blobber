package handler

import (
	"context"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	clientMap    = make(map[string]*ClientStats)
	mpLock       sync.RWMutex
	blackListMap = make(map[string]bool)
	blMap        sync.RWMutex
)

const (
	Period = 60 * 60 * 24 * 30 // 30 days
)

type ClientStats struct {
	ClientID      string           `gorm:"column:client_id;size:64;primaryKey" json:"client_id"`
	CreatedAt     common.Timestamp `gorm:"created_at;primaryKey" json:"created"`
	TotalUpload   int64            `gorm:"column:total_upload" json:"total_upload"`
	TotalDownload int64            `gorm:"column:total_download" json:"total_download"`
	TotalWM       int64            `gorm:"column:total_write_marker" json:"total_write_marker"`
	TotalZeroWM   int64            `gorm:"-" json:"total_zero_write_marker"`
}

func (ClientStats) TableName() string {
	return "client_stats"
}

func (cs *ClientStats) BeforeCreate(tx *gorm.DB) error {
	cs.CreatedAt = common.Now()
	return nil
}

func GetUploadedData(clientID string) int64 {
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
	cs.TotalUpload += data
}

func GetWriteMarkerCount(clientID string) int64 {
	mpLock.RLock()
	defer mpLock.RUnlock()
	cs := clientMap[clientID]
	if cs == nil {
		return 0
	}
	return cs.TotalWM
}

func AddWriteMarkerCount(clientID string, zeroSizeWM bool) {
	mpLock.Lock()
	defer mpLock.Unlock()
	cs := clientMap[clientID]
	if cs == nil {
		cs = &ClientStats{ClientID: clientID}
		clientMap[clientID] = cs
	}
	cs.TotalWM++
	if zeroSizeWM {
		cs.TotalZeroWM++
	}
	if cs.TotalZeroWM > config.Configuration.CommitZeroLimitDaily || cs.TotalWM > config.Configuration.CommitLimitDaily {
		logging.Logger.Info("Client blacklisted", zap.String("client_id", clientID), zap.Int64("total_write_marker", cs.TotalWM), zap.Int64("total_zero_write_marker", cs.TotalZeroWM), zap.Int64("commit_limit_daily", config.Configuration.CommitLimitDaily), zap.Int64("commit_zero_limit_daily", config.Configuration.CommitZeroLimitDaily))
		SetBlacklist(clientID)
	}
}

func SetBlacklist(clientID string) {
	blMap.Lock()
	blackListMap[clientID] = true
	blMap.Unlock()
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
	now := common.Now()
	for _, cs := range clientMap {
		cs.CreatedAt = now
		cs.TotalDownload = getDailyBlocks(cs.ClientID)
		dbStats = append(dbStats, cs)
	}
	clear(clientMap)
	mpLock.Unlock()
	_ = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		if len(dbStats) > 0 {
			tx := datastore.GetStore().GetTransaction(ctx)
			return tx.Create(dbStats).Error
		}
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
		clear(blackListMap)
		for _, clientID := range blackList {
			blackListMap[clientID] = true
		}
		blMap.Unlock()
	}
}

func startBlackListWorker(ctx context.Context) {
	logging.Logger.Info("Starting black list worker", zap.Int64("upload_limit", config.Configuration.UploadLimitMonthly), zap.Int64("download_limit", config.Configuration.BlockLimitMonthly), zap.Int64("commit_limit", config.Configuration.CommitLimitMonthly), zap.Int64("commit_zero_limit", config.Configuration.CommitZeroLimitDaily), zap.Int64("commit_limit_daily", config.Configuration.CommitLimitDaily))
	BlackListWorkerTime := 24 * time.Hour
	if config.Development() {
		BlackListWorkerTime = 10 * time.Second
	}
	saveClientStats()

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(BlackListWorkerTime):
			saveClientStats()
			cleanupDownloadLimit()
		}
	}
}
