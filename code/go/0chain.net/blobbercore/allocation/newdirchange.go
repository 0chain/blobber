package allocation

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/util"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

type NewDir struct {
	ConnectionID string `json:"connection_id" validation:"required"`
	Path         string `json:"filepath" validation:"required"`
	AllocationID string `json:"allocation_id"`
	CustomMeta   string `json:"custom_meta,omitempty"`
}

func (nf *NewDir) ApplyChange(ctx context.Context, rootRef *reference.Ref, change *AllocationChange,
	allocationRoot string, ts common.Timestamp, fileIDMeta map[string]string) (*reference.Ref, error) {

	totalRefs, err := reference.CountRefs(ctx, nf.AllocationID)
	if err != nil {
		return nil, err
	}

	if int64(config.Configuration.MaxAllocationDirFiles) <= totalRefs {
		return nil, common.NewErrorf("max_alloc_dir_files_reached",
			"maximum files and directories already reached: %v", err)
	}

	err = nf.Unmarshal(change.Input)
	if err != nil {
		return nil, err
	}

	if rootRef.CreatedAt == 0 {
		rootRef.CreatedAt = ts
	}
	rootRef.UpdatedAt = ts
	rootRef.HashToBeComputed = true
	fields, err := common.GetPathFields(nf.Path)
	if err != nil {
		return nil, err
	}

	dirRef := rootRef
	for i := 0; i < len(fields); i++ {
		found := false
		for _, child := range dirRef.Children {
			if child.Name == fields[i] {
				dirRef = child
				dirRef.HashToBeComputed = true
				dirRef.UpdatedAt = ts
				found = true
				break
			}
		}

		if !found {
			newRef := reference.NewDirectoryRef()
			newRef.AllocationID = nf.AllocationID
			newRef.Path = filepath.Join("/", strings.Join(fields[:i+1], "/"))
			newRef.PathLevel = len(fields) + 1
			newRef.ParentPath = filepath.Dir(newRef.Path)
			newRef.Name = fields[i]
			newRef.LookupHash = reference.GetReferenceLookup(nf.AllocationID, newRef.Path)
			newRef.CreatedAt = ts
			newRef.UpdatedAt = ts
			newRef.HashToBeComputed = true
			fileID, ok := fileIDMeta[newRef.Path]
			if !ok || fileID == "" {
				return nil, common.NewError("invalid_parameter",
					fmt.Sprintf("file path %s has no entry in fileID meta", newRef.Path))
			}
			newRef.FileID = fileID
			newRef.CustomMeta = nf.CustomMeta
			dirRef.AddChild(newRef)
			dirRef = newRef
		}
	}
	return rootRef, nil
}

func (nd *NewDir) Marshal() (string, error) {
	ret, err := json.Marshal(nd)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}

func (nd *NewDir) Unmarshal(input string) error {
	if err := json.Unmarshal([]byte(input), nd); err != nil {
		return err
	}

	return util.UnmarshalValidation(nd)
}

func (nf *NewDir) DeleteTempFile() error {
	return nil
}

func (nfch *NewDir) CommitToFileStore(ctx context.Context, mut *sync.Mutex) error {
	return nil
}

func (nfc *NewDir) GetPath() []string {
	return []string{nfc.Path}
}
