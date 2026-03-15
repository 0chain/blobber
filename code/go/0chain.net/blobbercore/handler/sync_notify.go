package handler

import (
	"encoding/json"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/util"
	"go.uber.org/zap"
)

func notifySyncEvent(allocationID, clientID, eventType string) {
	if !config.Configuration.SyncNotifyEnabled || config.Configuration.SyncNotifyURL == "" {
		return
	}

	Logger.Info("sync_notify_queued",
		zap.String("allocation_id", allocationID),
		zap.String("client_id", clientID),
		zap.String("event_type", eventType),
		zap.String("notify_url", config.Configuration.SyncNotifyURL))

	payload, err := json.Marshal(map[string]string{
		"allocation_id": allocationID,
		"client_id":     clientID,
		"event_type":    eventType,
	})
	if err != nil {
		Logger.Error("sync_notify_marshal_failed", zap.Error(err))
		return
	}

	go func() {
		resp, err := util.SendPostRequest(config.Configuration.SyncNotifyURL, payload, nil)
		if err != nil {
			Logger.Error("sync_notify_failed",
				zap.String("allocation_id", allocationID),
				zap.String("event_type", eventType),
				zap.Error(err))
		} else {
			Logger.Info("sync_notify_sent",
				zap.String("allocation_id", allocationID),
				zap.String("event_type", eventType),
				zap.String("response", string(resp)))
		}
	}()
}
