package allocation

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/util"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

// UploadFileChanger file change processor for continuous upload in INIT/APPEND/FINALIZE
type UploadFileChanger struct {
	BaseFileChanger
}

// ApplyChange update references, and create a new FileRef
func (nf *UploadFileChanger) ApplyChange(ctx context.Context, rootRef *reference.Ref, change *AllocationChange,
	allocationRoot string, ts common.Timestamp, fileIDMeta map[string]string) (*reference.Ref, error) {

	totalRefs, err := reference.CountRefs(ctx, nf.AllocationID)
	if err != nil {
		return nil, err
	}

	if int64(config.Configuration.MaxAllocationDirFiles) <= totalRefs {
		return nil, common.NewErrorf("max_alloc_dir_files_reached",
			"maximum files and directories already reached: %v", err)
	}

	fields, err := common.GetPathFields(filepath.Dir(nf.Path))
	if err != nil {
		return nil, err
	}
	if rootRef.CreatedAt == 0 {
		rootRef.CreatedAt = ts
	}

	rootRef.UpdatedAt = ts
	rootRef.HashToBeComputed = true

	dirRef := rootRef
	for i := 0; i < len(fields); i++ {
		found := false
		for _, child := range dirRef.Children {
			if child.Name == fields[i] {
				if child.Type != reference.DIRECTORY {
					return nil, common.NewError("invalid_reference_path", "Reference path has invalid ref type")
				}
				dirRef = child
				dirRef.UpdatedAt = ts
				dirRef.HashToBeComputed = true
				found = true
			}
		}
		if !found {
			newRef := reference.NewDirectoryRef()
			newRef.AllocationID = dirRef.AllocationID
			newRef.Path = "/" + strings.Join(fields[:i+1], "/")
			fileID, ok := fileIDMeta[newRef.Path]
			if !ok || fileID == "" {
				return nil, common.NewError("invalid_parameter",
					fmt.Sprintf("file path %s has no entry in fileID meta", newRef.Path))
			}
			newRef.FileID = fileID
			newRef.ParentPath = "/" + strings.Join(fields[:i], "/")
			newRef.Name = fields[i]
			newRef.CreatedAt = ts
			newRef.UpdatedAt = ts
			newRef.HashToBeComputed = true

			dirRef.AddChild(newRef)
			dirRef = newRef
		}
	}

	newFile := &reference.Ref{
		ActualFileHash:          nf.ActualHash,
		ActualFileHashSignature: nf.ActualFileHashSignature,
		ActualFileSize:          nf.ActualSize,
		AllocationID:            dirRef.AllocationID,
		ValidationRoot:          nf.ValidationRoot,
		ValidationRootSignature: nf.ValidationRootSignature,
		CustomMeta:              nf.CustomMeta,
		FixedMerkleRoot:         nf.FixedMerkleRoot,
		Name:                    nf.Filename,
		Path:                    nf.Path,
		ParentPath:              dirRef.Path,
		Type:                    reference.FILE,
		Size:                    nf.Size,
		MimeType:                nf.MimeType,
		AllocationRoot:          allocationRoot,
		ThumbnailHash:           nf.ThumbnailHash,
		ThumbnailSize:           nf.ThumbnailSize,
		ActualThumbnailHash:     nf.ActualThumbnailHash,
		ActualThumbnailSize:     nf.ActualThumbnailSize,
		EncryptedKey:            nf.EncryptedKey,
		ChunkSize:               nf.ChunkSize,
		CreatedAt:               ts,
		UpdatedAt:               ts,
		HashToBeComputed:        true,
		IsPrecommit:             true,
	}

	fileID, ok := fileIDMeta[newFile.Path]
	if !ok || fileID == "" {
		return nil, common.NewError("invalid_parameter",
			fmt.Sprintf("file path %s has no entry in fileID meta", newFile.Path))
	}
	newFile.FileID = fileID

	dirRef.AddChild(newFile)

	return rootRef, nil
}

// Marshal marshal and change to persistent to postgres
func (nf *UploadFileChanger) Marshal() (string, error) {
	ret, err := json.Marshal(nf)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}

// Unmarshal reload and unmarshal change from allocation_changes.input on postgres
func (nf *UploadFileChanger) Unmarshal(input string) error {
	if err := json.Unmarshal([]byte(input), nf); err != nil {
		return err
	}

	return util.UnmarshalValidation(nf)
}

func (nf *UploadFileChanger) GetPath() []string {
	return []string{nf.Path}
}
