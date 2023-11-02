package main

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/zcncore"
)

type storageScCB struct {
	done chan struct{}
	cct  int64
	mfs  int64
	err  error
}

func (ssc *storageScCB) OnInfoAvailable(op int, status int, info string, errStr string) {
	defer func() {
		ssc.done <- struct{}{}
	}()

	if errStr != "" {
		ssc.err = errors.New(errStr)
		return
	}

	m := make(map[string]interface{})
	err := json.Unmarshal([]byte(info), &m)
	if err != nil {
		ssc.err = err
		return
	}

	m = m["fields"].(map[string]interface{})

	cctString := m["max_challenge_completion_rounds"].(string)
	mfsString := m["max_file_size"].(string)

	cct, err := strconv.ParseInt(cctString, 10, 64)
	if err != nil {
		ssc.err = err
		return
	}

	mfs, err := strconv.ParseInt(mfsString, 10, 64)
	if err != nil {
		ssc.err = err
		return
	}

	ssc.cct = cct
	ssc.mfs = mfs
}

func setStorageScConfigFromChain() error {
	cb := &storageScCB{
		done: make(chan struct{}),
	}
	err := zcncore.GetStorageSCConfig(cb)
	if err != nil {
		return err
	}
	<-cb.done
	if cb.err != nil {
		return err
	}

	config.StorageSCConfig.ChallengeCompletionTime = cb.cct
	config.StorageSCConfig.MaxFileSize = cb.mfs
	return nil
}

func updateStorageScConfigWorker(ctx context.Context) {
	interval := time.Hour
	if config.Development() {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := setStorageScConfigFromChain()
			if err != nil {
				logging.Logger.Error(err.Error())
			}
		}
	}
}
