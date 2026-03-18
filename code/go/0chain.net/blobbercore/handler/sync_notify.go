package handler

import (
	"encoding/json"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/util"
)

func notifySyncEvent(allocationID, clientID, eventType string) {
	if !config.Configuration.SyncNotifyEnabled || config.Configuration.SyncNotifyURL == "" {
		return
	}

	payload, err := json.Marshal(map[string]string{
		"allocation_id": allocationID,
		"client_id":     clientID,
		"event_type":    eventType,
	})
	if err != nil {
		return
	}

	go func() {
		util.SendPostRequest(config.Configuration.SyncNotifyURL, payload, nil)
	}()
}
