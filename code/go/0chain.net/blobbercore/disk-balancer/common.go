package disk_balancer

import (
	"encoding/json"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

func deleteAllocation(path string) {
	timer := time.NewTicker(5 * time.Minute)
	ctx := common.GetRootContext()
	for {
		select {
		case <-ctx.Done():
			break
		case <-timer.C:
			if err := os.RemoveAll(path); err != nil {
				Logger.Error("deleteAllocation() failed to remove old allocation", zap.Error(err))
			}
			timer.Stop()
			break
		}
	}
}

func readFile(file string) *allocationInfo {
	data, _ := os.ReadFile(file)
	allocation := &allocationInfo{}
	_ = json.Unmarshal(data, allocation)
	return allocation
}
