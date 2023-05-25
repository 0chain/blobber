package allocation

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

type MoveFileChange struct {
	ConnectionID string `json:"connection_id"`
	AllocationID string `json:"allocation_id"`
	SrcPath      string `json:"path"`
	DestPath     string `json:"dest_path"`
}

func (rf *MoveFileChange) DeleteTempFile() error {
	return nil
}

func (rf *MoveFileChange) ApplyChange(ctx context.Context, change *AllocationChange,
	allocationRoot string, ts common.Timestamp, _ map[string]string) (*reference.Ref, error) {

	srcRef, err := reference.GetObjectTree(ctx, rf.AllocationID, rf.SrcPath)
	if err != nil {
		return nil, err
	}

	rootRef, err := reference.GetReferenceForHashCalculationFromPaths(ctx, rf.AllocationID,
		[]string{rf.DestPath, filepath.Dir(rf.SrcPath)})

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
			newRef.ParentPath = filepath.Join("/", strings.Join(fields[:i], "/"))
			newRef.Name = fields[i]
			newRef.HashToBeComputed = true
			newRef.CreatedAt = ts
			newRef.UpdatedAt = ts
			dirRef.AddChild(newRef)
			dirRef = newRef
		}
	}

	fileRefs := rf.processMoveRefs(ctx, srcRef, dirRef, allocationRoot, ts, true)

	srcParentPath, srcFileName := filepath.Split(rf.SrcPath)
	srcFields, err := common.GetPathFields(srcParentPath)
	if err != nil {
		return nil, err
	}
	dirRef = rootRef
	for i := 0; i < len(srcFields); i++ {
		found := false
		for _, child := range dirRef.Children {
			if child.Name == srcFields[i] {
				dirRef = child
				found = true
				dirRef.HashToBeComputed = true
				break
			}
		}
		if !found {
			return nil, common.NewError("invalid_reference_path",
				fmt.Sprintf("path %s does not exist", strings.Join(srcFields[:i+1], "/")))
		}
	}

	var removed bool
	for i, child := range dirRef.Children {
		if child.Name == srcFileName {
			dirRef.RemoveChild(i)
			removed = true
		}
	}
	if !removed {
		return nil, common.NewError("incomplete_move",
			"move operation rejected as it cannot be completed")
	}
	_, err = rootRef.CalculateHash(ctx, true)
	if err != nil {
		return nil, err
	}

	for _, fileRef := range fileRefs {
		fileRef.IsPrecommit = true
	}
	return rootRef, err
}

func (rf *MoveFileChange) processMoveRefs(
	ctx context.Context, srcRef, destRef *reference.Ref,
	allocationRoot string, ts common.Timestamp, toAdd bool) (fileRefs []*reference.Ref) {

	if srcRef.Type == reference.DIRECTORY {
		srcRef.Path = filepath.Join(destRef.Path, srcRef.Name)
		srcRef.ParentPath = destRef.Path
		srcRef.UpdatedAt = ts
		srcRef.HashToBeComputed = true
		if toAdd {
			destRef.AddChild(srcRef)
		}

		for _, childRef := range srcRef.Children {
			fileRefs = append(fileRefs, rf.processMoveRefs(ctx, childRef, srcRef, allocationRoot, ts, false)...)
		}
	} else if srcRef.Type == reference.FILE {
		srcRef.ParentPath = destRef.Path
		srcRef.Path = filepath.Join(destRef.Path, srcRef.Name)
		srcRef.UpdatedAt = ts
		srcRef.HashToBeComputed = true
		if toAdd {
			destRef.AddChild(srcRef)
		}
		fileRefs = append(fileRefs, srcRef)
	}

	return

}

func (rf *MoveFileChange) Marshal() (string, error) {
	ret, err := json.Marshal(rf)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}

func (rf *MoveFileChange) Unmarshal(input string) error {
	err := json.Unmarshal([]byte(input), rf)
	return err
}

func (rf *MoveFileChange) CommitToFileStore(ctx context.Context) error {
	return nil
}
