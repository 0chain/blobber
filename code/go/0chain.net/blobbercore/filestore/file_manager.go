package filestore

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
)

type FileStore struct {
	mp      string // mount point
	mAllocs map[string]*allocation

	allocMu *sync.Mutex
	rwMU    *sync.RWMutex

	diskCapacity uint64
}

var contentHashMapLock = common.GetLocker()

func getKey(allocID, contentHash string) string {
	return encryption.Hash(allocID + contentHash)
}

func (fs *FileStore) Initialize() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	fs.mp = config.Configuration.MountPoint
	if !fs.isMountPoint() {
		return fmt.Errorf("%s is not mount point", fs.mp)
	}

	if err = validateDirLevels(); err != nil {
		return
	}

	fs.allocMu = &sync.Mutex{}
	fs.rwMU = &sync.RWMutex{}
	fs.mAllocs = make(map[string]*allocation)

	if err = fs.initMap(); err != nil {
		return
	}

	return nil
}

func (fs *FileStore) IterateObjects(allocationID string, handler FileObjectHandler) error {
	allocDir := fs.getAllocDir(allocationID)
	tmpPrefix := filepath.Join(allocDir, TempDir)
	return filepath.Walk(allocDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && !strings.HasPrefix(path, tmpPrefix) {
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			h := sha256.New()
			if _, err := io.Copy(h, f); err != nil {
				return nil
			}
			handler(hex.EncodeToString(h.Sum(nil)), info.Size())
		}
		return nil
	})
}
