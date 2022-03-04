package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/zboxcore/fileref"
	"go.uber.org/zap"
)

// AddFileCommand command for resuming file
type AddFileCommand struct {
	existingFileRef  *reference.Ref
	allocationChange *allocation.AllocationChange
	fileChanger      *allocation.AddFileChanger
}

// IsAuthorized validate request.
func (cmd *AddFileCommand) IsAuthorized(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, clientID string) error {
	fmt.Println("Start Is Authorized !!!")
	defer func() {
		fmt.Println("End Is Authorized !!!")
	}()
	if allocationObj.OwnerID != clientID && allocationObj.RepairerID != clientID {
		return common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
	}

	fileChanger := &allocation.AddFileChanger{}

	uploadMetaString := req.FormValue("uploadMeta")
	err := json.Unmarshal([]byte(uploadMetaString), fileChanger)
	fmt.Println("IsAuthorized : uploadMetaString: ", uploadMetaString)
	if err != nil {
		return common.NewError("invalid_parameters",
			"Invalid parameters. Error parsing the meta data for upload."+err.Error())
	}
	// Update GetReference to GetReferenceID
	cmd.existingFileRef, _ = reference.GetReferenceID(ctx, allocationObj.ID, fileChanger.Path)

	if cmd.existingFileRef != nil {
		return common.NewError("duplicate_file", "File at path already exists")
	}

	//create a FixedMerkleTree instance first, it will be reloaded from db in cmd.reloadChange if it is not first chunk
	//cmd.fileChanger.FixedMerkleTree = &util.FixedMerkleTree{}

	if fileChanger.ChunkSize <= 0 {
		fileChanger.ChunkSize = fileref.CHUNK_SIZE
	}

	cmd.fileChanger = fileChanger

	return nil
}

// ProcessContent flush file to FileStorage
func (cmd *AddFileCommand) ProcessContent(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) (blobberhttp.UploadResult, error) {
	result := blobberhttp.UploadResult{}

	origfile, _, err := req.FormFile("uploadFile")
	if err != nil {
		return result, common.NewError("invalid_parameters", "Error Reading multi parts for file."+err.Error())
	}
	defer origfile.Close()

	cmd.reloadChange(connectionObj)

	fileInputData := &filestore.FileInputData{
		Name:    cmd.fileChanger.Filename,
		Path:    cmd.fileChanger.Path,
		OnCloud: false,

		ChunkSize:    cmd.fileChanger.ChunkSize,
		UploadOffset: cmd.fileChanger.UploadOffset,
		IsChunked:    true,
		IsFinal:      cmd.fileChanger.IsFinal,
	}
	fileOutputData, err := filestore.GetFileStore().WriteFile(allocationObj.ID, fileInputData, origfile, connectionObj.ConnectionID)
	if err != nil {
		return result, common.NewError("upload_error", "Failed to upload the file. "+err.Error())
	}

	result.Filename = cmd.fileChanger.Filename
	result.Hash = fileOutputData.ContentHash
	//result.MerkleRoot = fileOutputData.MerkleRoot
	result.Size = fileOutputData.Size

	allocationSize := connectionObj.Size

	// only update connection size when the chunk is uploaded by first time.
	if !fileOutputData.ChunkUploaded {
		allocationSize += fileOutputData.Size
	}

	if allocationObj.BlobberSizeUsed+allocationSize > allocationObj.BlobberSize {
		return result, common.NewError("max_allocation_size", "Max size reached for the allocation with this blobber")
	}

	if len(cmd.fileChanger.ChunkHash) > 0 && cmd.fileChanger.ChunkHash != fileOutputData.ContentHash {
		return result, common.NewError("content_hash_mismatch", "Content hash provided in the meta data does not match the file content")
	}

	// Save client's ContentHash in database instead blobber's
	// it saves time to read and compute hash of fragment from disk again
	//cmd.fileChanger.Hash = fileOutputData.ContentHash

	cmd.fileChanger.AllocationID = allocationObj.ID
	cmd.fileChanger.Size = allocationSize

	cmd.allocationChange = &allocation.AllocationChange{}
	cmd.allocationChange.ConnectionID = connectionObj.ConnectionID
	cmd.allocationChange.Size = allocationSize
	cmd.allocationChange.Operation = constants.FileOperationInsert

	connectionObj.Size = allocationSize

	return result, nil
}

// ProcessThumbnail flush thumbnail file to FileStorage if it has.
func (cmd *AddFileCommand) ProcessThumbnail(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) error {
	thumbfile, thumbHeader, _ := req.FormFile("uploadThumbnailFile")

	if thumbHeader != nil {
		defer thumbfile.Close()

		thumbInputData := &filestore.FileInputData{Name: thumbHeader.Filename, Path: cmd.fileChanger.Path}
		thumbOutputData, err := filestore.GetFileStore().WriteFile(allocationObj.ID, thumbInputData, thumbfile, connectionObj.ConnectionID)
		if err != nil {
			return common.NewError("upload_error", "Failed to upload the thumbnail. "+err.Error())
		}
		if cmd.fileChanger.ThumbnailHash != thumbOutputData.ContentHash {
			return common.NewError("content_hash_mismatch", "Content hash provided in the meta data does not match the thumbnail content")
		}

		cmd.fileChanger.ThumbnailHash = thumbOutputData.ContentHash
		cmd.fileChanger.ThumbnailSize = thumbOutputData.Size
		cmd.fileChanger.ThumbnailFilename = thumbInputData.Name
	}

	return nil
}

func (cmd *AddFileCommand) reloadChange(connectionObj *allocation.AllocationChangeCollector) {
	for _, c := range connectionObj.Changes {
		if c.Operation != constants.FileOperationInsert {
			continue
		}

		dbChangeProcessor := &allocation.AddFileChanger{}

		err := dbChangeProcessor.Unmarshal(c.Input)
		if err != nil {
			logging.Logger.Error("reloadChange", zap.Error(err))
		}

		cmd.fileChanger.Size = dbChangeProcessor.Size
		cmd.fileChanger.ThumbnailFilename = dbChangeProcessor.ThumbnailFilename
		cmd.fileChanger.ThumbnailSize = dbChangeProcessor.ThumbnailSize
		cmd.fileChanger.ThumbnailHash = dbChangeProcessor.Hash

		return
	}
}

// UpdateChange replace AddFileChange in db
func (cmd *AddFileCommand) UpdateChange(ctx context.Context, connectionObj *allocation.AllocationChangeCollector) error {
	for _, c := range connectionObj.Changes {
		if c.Operation != constants.FileOperationInsert {
			continue
		}
		c.Size = connectionObj.Size
		c.Input, _ = cmd.fileChanger.Marshal()

		//c.ModelWithTS.UpdatedAt = time.Now()
		err := connectionObj.Save(ctx)
		if err != nil {
			return err
		}

		return c.Save(ctx)
	}

	//NOT FOUND
	connectionObj.AddChange(cmd.allocationChange, cmd.fileChanger)

	return connectionObj.Save(ctx)
}
