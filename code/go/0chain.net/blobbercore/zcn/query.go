package zcn

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/0chain/gosdk/zcncore"
)

var ErrBlobberNotFound = errors.New("blobber is not found on chain")

// GetBlobber try to get blobber info from chain.
func GetBlobber(blobberID string) (*zcncore.Blobber, error) {
	cb := &getBlobberCallback{}
	cb.wg.Add(1)
	if err := zcncore.GetBlobber(blobberID, cb); err != nil {
		cb.wg.Done()
		return nil, err
	}

	cb.wg.Wait()
	if cb.Error != nil {
		return nil, cb.Error
	}

	if cb.Blobber == nil {
		return nil, ErrBlobberNotFound
	}

	return cb.Blobber, nil

}

type getBlobberCallback struct {
	wg      sync.WaitGroup
	Blobber *zcncore.Blobber
	Error   error
}

func (cb *getBlobberCallback) OnInfoAvailable(op int, status int, info string, err string) {
	defer cb.wg.Done()
	if status != zcncore.StatusSuccess {
		cb.Error = errors.New(err)
		return
	}
	b := &zcncore.Blobber{}
	if err := json.Unmarshal([]byte(info), b); err != nil {
		cb.Error = fmt.Errorf("getBlobber:json %s %w", info, err)
		return
	}

	cb.Blobber = b

}
