package allocation

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

type CopyFileChange struct {
	ConnectionID string `json:"connection_id"`
	AllocationID string `json:"allocation_id"`
	SrcPath      string `json:"path"`
	DestPath     string `json:"dest_path"`
}

func (rf *CopyFileChange) DeleteTempFile() error {
	return nil
}

func (rf *CopyFileChange) ApplyChange(ctx context.Context, rootRef *reference.Ref, change *AllocationChange,
	allocationRoot string, ts common.Timestamp, fileIDMeta map[string]string) (*reference.Ref, error) {

	totalRefs, err := reference.CountRefs(rf.AllocationID)
	if err != nil {
		return nil, err
	}

	if int64(config.Configuration.MaxAllocationDirFiles) <= totalRefs {
		return nil, common.NewErrorf("max_alloc_dir_files_reached",
			"maximum files and directories already reached: %v", err)
	}

	srcRef, err := rootRef.GetSrcPath(rf.SrcPath)
	if err != nil {
		return nil, err
	}

	rootRef.UpdatedAt = ts
	rootRef.HashToBeComputed = true

	dirRef := rootRef
	fields, err := common.GetPathFields(rf.DestPath)
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(fields); i++ {
		found := false
		for _, child := range dirRef.Children {
			if child.Name == fields[i] {
				if child.Type == reference.DIRECTORY {
					child.HashToBeComputed = true
					dirRef = child
					dirRef.UpdatedAt = ts
					found = true
				} else {
					return nil, common.NewError("invalid_path",
						fmt.Sprintf("%s is of file type", child.Path))
				}
			}
		}

		if !found {
			newRef := reference.NewDirectoryRef()
			newRef.AllocationID = rf.AllocationID
			newRef.Path = filepath.Join("/", strings.Join(fields[:i+1], "/"))
			fileID, ok := fileIDMeta[newRef.Path]
			if !ok || fileID == "" {
				return nil, common.NewError("invalid_parameter",
					fmt.Sprintf("file path %s has no entry in file ID meta", newRef.Path))
			}
			newRef.FileID = fileID
			newRef.ParentPath = filepath.Join("/", strings.Join(fields[:i], "/"))
			newRef.Name = fields[i]
			newRef.HashToBeComputed = true
			newRef.CreatedAt = ts
			newRef.UpdatedAt = ts
			dirRef.AddChild(newRef)
			dirRef = newRef
		}
	}

	_, err = rf.processCopyRefs(ctx, srcRef, dirRef, allocationRoot, ts, fileIDMeta)
	if err != nil {
		return nil, err
	}

	// _, err = rootRef.CalculateHash(ctx, true)
	// if err != nil {
	// 	return nil, err
	// }

	// for _, fileRef := range fileRefs {
	// 	stats.NewFileCreated(ctx, fileRef.ID)
	// }
	return rootRef, err
}

func (rf *CopyFileChange) processCopyRefs(
	ctx context.Context, srcRef, destRef *reference.Ref,
	allocationRoot string, ts common.Timestamp, fileIDMeta map[string]string,
) (
	fileRefs []*reference.Ref, err error,
) {

	newRef := *srcRef
	newRef.ID = 0
	newRef.Path = filepath.Join(destRef.Path, srcRef.Name)
	fileID, ok := fileIDMeta[newRef.Path]
	if !ok || fileID == "" {
		return nil, common.NewError("invalid_parameter",
			fmt.Sprintf("file path %s has no entry in fileID meta", newRef.Path))
	}
	newRef.FileID = fileID
	newRef.ParentPath = destRef.Path
	newRef.CreatedAt = ts
	newRef.UpdatedAt = ts
	newRef.HashToBeComputed = true
	destRef.AddChild(&newRef)
	if newRef.Type == reference.DIRECTORY {
		for _, childRef := range srcRef.Children {
			fRefs, err := rf.processCopyRefs(ctx, childRef, &newRef, allocationRoot, ts, fileIDMeta)
			if err != nil {
				return nil, err
			}
			fileRefs = append(fileRefs, fRefs...)
		}
	} else {
		fileRefs = append(fileRefs, &newRef)
	}

	return
}

func (rf *CopyFileChange) Marshal() (string, error) {
	ret, err := json.Marshal(rf)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}

func (rf *CopyFileChange) Unmarshal(input string) error {
	err := json.Unmarshal([]byte(input), rf)
	return err
}

func (rf *CopyFileChange) CommitToFileStore(ctx context.Context) error {
	return nil
}

func (rf *CopyFileChange) GetPath() []string {
	return []string{rf.DestPath, rf.SrcPath}
}
