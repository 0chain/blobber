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

type cctCB struct {
	done chan struct{}
	cct  int64
	err  error
}

type maxFileSizeCB struct {
	done chan struct{}
	mfs  int64
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

	cctString := m["max_challenge_completion_rounds"].(string)

	cct, err := strconv.ParseInt(cctString, 10, 64)
	if err != nil {
		c.err = err
		return
	}

	c.cct = cct
}

func (c *maxFileSizeCB) OnInfoAvailable(op int, status int, info string, errStr string) {
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

	mfsString := m["max_file_size"].(string)

	mfs, err := strconv.ParseInt(mfsString, 10, 64)
	if err != nil {
		c.err = err
		return
	}

	c.mfs = mfs
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

func setMaxFileSizeFromChain() error {
	cb := &maxFileSizeCB{
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

	config.StorageSCConfig.MaxFileSize = cb.mfs
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
