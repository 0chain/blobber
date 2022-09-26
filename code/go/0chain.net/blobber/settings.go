package main

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/zcncore"
)

type cctCB struct {
	done chan struct{}
	cct  time.Duration
	err  error
}

func (c *cctCB) OnInfoAvailable(op int, status int, info string, errStr string) {
	defer func() {
		c.done <- struct{}{}
	}()

	if errStr != "" {
		c.err = errors.New(errStr)
		return
	}

	m := make(map[string]interface{})
	err := json.Unmarshal([]byte(info), &m)
	if err != nil {
		c.err = err
		return
	}

	m = m["fields"].(map[string]interface{})
	cct := m["challenge_completion_time"].(string)

	d, err := time.ParseDuration(cct)
	if err != nil {
		c.err = err
		return
	}
	c.cct = d
}

func setCCTFromChain() error {
	cb := &cctCB{
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
	return nil
}

func updateCCTWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// We'd panic if err occurred when calling setCCTFromChain from
			// main.go file because cct would be initially 0 and we cannot
			// work with 0 value.
			// Upon updating cct, we only log error because cct is not 0
			// We should try to submit challenge as soon as possible regardless
			// of cct value.
			err := setCCTFromChain()
			if err != nil {
				logging.Logger.Error(err.Error())
			}
		}
	}
}
