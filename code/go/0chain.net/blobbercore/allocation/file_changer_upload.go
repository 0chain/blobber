package allocation

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/util"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

// UploadFileChanger file change processor for continuous upload in INIT/APPEND/FINALIZE
type UploadFileChanger struct {
	BaseFileChanger
}

// ApplyChange update references, and create a new FileRef
func (nf *UploadFileChanger) applyChange(ctx context.Context,
	ts common.Timestamp, fileIDMeta map[string]string, collector reference.QueryCollector) error {

	parentPath, _ := filepath.Split(nf.Path)
	nf.LookupHash = reference.GetReferenceLookup(nf.AllocationID, nf.Path)
	newFile := &reference.Ref{
		ActualFileHash:          nf.ActualHash,
		ActualFileHashSignature: nf.ActualFileHashSignature,
		ActualFileSize:          nf.ActualSize,
		AllocationID:            nf.AllocationID,
		CustomMeta:              nf.CustomMeta,
		Name:                    nf.Filename,
		Path:                    nf.Path,
		ParentPath:              parentPath,
		Type:                    reference.FILE,
		Size:                    nf.Size,
		MimeType:                nf.MimeType,
		ThumbnailHash:           nf.ThumbnailHash,
		ThumbnailSize:           nf.ThumbnailSize,
		ActualThumbnailHash:     nf.ActualThumbnailHash,
		ActualThumbnailSize:     nf.ActualThumbnailSize,
		EncryptedKey:            nf.EncryptedKey,
		EncryptedKeyPoint:       nf.EncryptedKeyPoint,
		ChunkSize:               nf.ChunkSize,
		CreatedAt:               ts,
		UpdatedAt:               ts,
		HashToBeComputed:        true,
		IsPrecommit:             true,
		LookupHash:              nf.LookupHash,
		DataHash:                nf.DataHash,
		DataHashSignature:       nf.DataHashSignature,
		FilestoreVersion:        filestore.VERSION,
	}

	fileID, ok := fileIDMeta[newFile.Path]
	if !ok || fileID == "" {
		return common.NewError("invalid_parameter",
			fmt.Sprintf("file path %s has no entry in fileID meta", newFile.Path))
	}
	newFile.FileID = fileID

	return nil
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
