package allocation

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/util"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

// AddFileChanger file change processor for continuous upload in INIT/APPEND/FINALIZE
type AddFileChanger struct {
	BaseFileChanger
}

// ProcessChange update references, and create a new FileRef
func (nf *AddFileChanger) ProcessChange(ctx context.Context, change *AllocationChange, allocationRoot string) (*reference.Ref, error) {

	path, _ := filepath.Split(nf.Path)
	path = filepath.Clean(path)
	tSubDirs := reference.GetSubDirsFromPath(path)

	// Maybe Change this from GetReferencePath to GetReferencePath2
	rootRef, err := reference.GetReferencePath2(ctx, nf.AllocationID, nf.Path)
	if err != nil {
		return nil, err
	}
	dirRef := rootRef
	treelevel := 0
	for {
		found := false
		for _, child := range dirRef.Children {
			if child.Type == reference.DIRECTORY && treelevel < len(tSubDirs) {
				if child.Name == tSubDirs[treelevel] {
					dirRef = child
					found = true
					break
				}
			}
		}
		if found {
			treelevel++
			continue
		}
		if len(tSubDirs) > treelevel {
			newRef := reference.NewDirectoryRef()
			newRef.AllocationID = dirRef.AllocationID
			newRef.Path = "/" + strings.Join(tSubDirs[:treelevel+1], "/")
			newRef.ParentPath = "/" + strings.Join(tSubDirs[:treelevel], "/")
			newRef.Name = tSubDirs[treelevel]
			newRef.LookupHash = reference.GetReferenceLookup(dirRef.AllocationID, newRef.Path)
			dirRef.AddChild(newRef)
			dirRef = newRef
			treelevel++
			continue
		} else {
			break
		}
	}

	var newFile = reference.NewFileRef()
	newFile.ActualFileHash = nf.ActualHash
	newFile.ActualFileSize = nf.ActualSize
	newFile.AllocationID = dirRef.AllocationID
	newFile.ContentHash = nf.Hash
	newFile.CustomMeta = nf.CustomMeta
	newFile.MerkleRoot = nf.MerkleRoot
	newFile.Name = nf.Filename
	newFile.ParentPath = dirRef.Path
	newFile.Path = nf.Path
	newFile.LookupHash = reference.GetReferenceLookup(dirRef.AllocationID, nf.Path)
	newFile.Size = nf.Size
	newFile.MimeType = nf.MimeType
	newFile.WriteMarker = allocationRoot
	newFile.ThumbnailHash = nf.ThumbnailHash
	newFile.ThumbnailSize = nf.ThumbnailSize
	newFile.ActualThumbnailHash = nf.ActualThumbnailHash
	newFile.ActualThumbnailSize = nf.ActualThumbnailSize
	newFile.EncryptedKey = nf.EncryptedKey
	newFile.ChunkSize = nf.ChunkSize
	if err = newFile.SetAttributes(&nf.Attributes); err != nil {
		return nil, common.NewErrorf("process_new_file_change",
			"setting file attributes: %v", err)
	}
	dirRef.AddChild(newFile)
	fmt.Println("File Changer Add !!! Adding New File Data So we need to Make a Create in Calculate Hash and DB Save !!!")
	if _, err := rootRef.CalculateHash(ctx, true); err != nil {
		return nil, err
	}
	stats.NewFileCreated(ctx, newFile.ID)
	return rootRef, nil
}

// Marshal marshal and change to persistent to postgres
func (nf *AddFileChanger) Marshal() (string, error) {
	ret, err := json.Marshal(nf)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}

// Unmarshal reload and unmarshal change from allocation_changes.input on postgres
func (nf *AddFileChanger) Unmarshal(input string) error {
	if err := json.Unmarshal([]byte(input), nf); err != nil {
		return err
	}

	return util.UnmarshalValidation(nf)
}
