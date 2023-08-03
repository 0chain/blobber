package conductrpc

import (
	"context"

	"log"

	"github.com/0chain/blobber/code/go/0chain.net/core/node"
)

// alreadyRunning is simple indicator that given function is running
// no need to acquire mutex lock. It does not matter if at a time it
// somehow runs the given function multiple times. Since it takes some
// time to acquire state from rpc server there is no concurrent running
var alreadyRunning bool

func SendFileMetaRoot() {
	if alreadyRunning {
		return
	}
	alreadyRunning = true
	defer func() {
		alreadyRunning = false
	}()

	ctx, ctxCncl := context.WithCancel(context.TODO())
	defer ctxCncl()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		s := global.State()
		if s.GetFileMetaRoot != nil {
			fmr, err := getFileMetaRoot()
			if err != nil {
				log.Printf("Error: %v", err)
				continue
			}

			global.SendFileMetaRoot(node.Self.ID, fmr, ctxCncl)
		}
	}
}

func getFileMetaRoot() (string, error) {
	return "", nil
}
