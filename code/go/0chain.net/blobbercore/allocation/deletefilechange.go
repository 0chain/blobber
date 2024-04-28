package allocation

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"gorm.io/gorm"

	"go.uber.org/zap"
)

var (
	NoNewLock = common.NewError("not_new_lock", "")
)

type DeleteFileChange struct {
	ConnectionID string `json:"connection_id"`
	AllocationID string `json:"allocation_id"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	Hash         string `json:"hash"`
}

func (nf *DeleteFileChange) ApplyChange(ctx context.Context, rootRef *reference.Ref, change *AllocationChange,
	allocationRoot string, ts common.Timestamp, _ map[string]string) (*reference.Ref, error) {

	err := reference.DeleteObject(ctx, rootRef, nf.AllocationID, filepath.Clean(nf.Path), ts)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (nf *DeleteFileChange) Marshal() (string, error) {
	ret, err := json.Marshal(nf)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}

func (nf *DeleteFileChange) Unmarshal(input string) error {
	err := json.Unmarshal([]byte(input), nf)
	return err
}

func (nf *DeleteFileChange) DeleteTempFile() error {
	return nil
}

func (nf *DeleteFileChange) CommitToFileStore(ctx context.Context, mut *sync.Mutex) error {
	db := datastore.GetStore().GetTransaction(ctx)
	type Result struct {
		Id               string
		ValidationRoot   string
		ThumbnailHash    string
		FileStoreVersion int
	}

	limitCh := make(chan struct{}, 10)
	wg := &sync.WaitGroup{}
	var results []Result
	mut.Lock()
	err := db.Model(&reference.Ref{}).Unscoped().
		Select("id", "validation_root", "thumbnail_hash", "filestore_version").
		Where("allocation_id=? AND path LIKE ? AND type=? AND deleted_at is not NULL",
			nf.AllocationID, nf.Path+"%", reference.FILE).
		FindInBatches(&results, 100, func(tx *gorm.DB, batch int) error {

			for _, res := range results {
				var count int64
				tx.Model(&reference.Ref{}).
					Where("allocation_id=? AND validation_root=?", nf.AllocationID, res.ValidationRoot).
					Count(&count)

				if count != 0 && res.ThumbnailHash == "" {
					continue
				}

				limitCh <- struct{}{}
				wg.Add(1)

				go func(res Result, count int64) {
					defer func() {
						<-limitCh
						wg.Done()
					}()

					if count == 0 {
						err := filestore.GetFileStore().DeleteFile(nf.AllocationID, res.ValidationRoot, res.FileStoreVersion)
						if err != nil {
							logging.Logger.Error(fmt.Sprintf("Error while deleting file: %s", err.Error()),
								zap.String("validation_root", res.ValidationRoot))
						}
					}
					// We don't increase alloc size for thumbnail so we don't need to decrease it
					// if res.ThumbnailHash != "" {
					// 	err := filestore.GetFileStore().DeleteFile(nf.AllocationID, res.ThumbnailHash)
					// 	if err != nil {
					// 		logging.Logger.Error(fmt.Sprintf("Error while deleting thumbnail: %s", err.Error()),
					// 			zap.String("thumbnail", res.ThumbnailHash))
					// 	}
					// }

				}(res, count)

			}
			return nil
		}).Error
	mut.Unlock()
	wg.Wait()

	return err
	// return db.Model(&reference.Ref{}).Unscoped().
	// 	Delete(&reference.Ref{},
	// 		"allocation_id = ? AND path LIKE ? AND deleted_at IS NOT NULL",
	// 		nf.AllocationID, nf.Path+"%").Error
}

func (nf *DeleteFileChange) GetPath() []string {
	return []string{nf.Path}
}
