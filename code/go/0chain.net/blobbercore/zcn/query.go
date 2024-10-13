package zcn

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/0chain/gosdk/zboxcore/sdk"
	"sync"

	"github.com/0chain/gosdk/zcncore"
)

var ErrBlobberNotFound = errors.New("blobber is not found on chain")

// GetBlobber try to get blobber info from chain.
func GetBlobber(blobberID string) (*sdk.Blobber, error) {
	var (
		blobber *sdk.Blobber
		err     error
	)
	if blobber, err = sdk.GetBlobber(blobberID); err != nil {
		return nil, err
	}

	return blobber, nil

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
