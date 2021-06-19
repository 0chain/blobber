package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"0chain.net/blobbercore/allocation"
	"0chain.net/blobbercore/config"
	"0chain.net/blobbercore/filestore"
	"0chain.net/blobbercore/reference"
	"0chain.net/core/common"
)

// ResumeFileCommand command for resuming file
type ResumeFileCommand struct {
	allocationChange *allocation.AllocationChange
	changeProcessor  *allocation.ResumeFileChange
}

// IsAuthorized validate request.
func (cmd *ResumeFileCommand) IsAuthorized(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, clientID string) error {
	if allocationObj.OwnerID != clientID && allocationObj.PayerID != clientID {
		return common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
	}

	changeProcessor := &allocation.ResumeFileChange{}

	uploadMetaString := req.FormValue("uploadMeta")
	err := json.Unmarshal([]byte(uploadMetaString), changeProcessor)
	if err != nil {
		return common.NewError("invalid_parameters",
			"Invalid parameters. Error parsing the meta data for upload."+err.Error())
	}
	exisitingFileRef, _ := reference.GetReference(ctx, allocationObj.ID, changeProcessor.Path)

	if exisitingFileRef != nil {
		return common.NewError("duplicate_file", "File at path already exists")
	}

	//create a TrustedConentHasher instance first, it will be reloaded from db in cmd.reloadChange if it is not first chunk
	//cmd.changeProcessor.TrustedConentHasher = &util.TrustedConentHasher{}

	cmd.changeProcessor = changeProcessor

	return nil

}

// ProcessContent flush file to FileStorage
func (cmd *ResumeFileCommand) ProcessContent(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) (UploadResult, error) {
	result := UploadResult{}

	origfile, _, err := req.FormFile("uploadFile")
	if err != nil {
		return result, common.NewError("invalid_parameters", "Error Reading multi parts for file."+err.Error())
	}
	defer origfile.Close()

	cmd.reloadChange(connectionObj)

	fileInputData := &filestore.FileInputData{Name: cmd.changeProcessor.Filename,
		Path:         cmd.changeProcessor.Path,
		OnCloud:      false,
		UploadOffset: cmd.changeProcessor.UploadOffset,
		IsResumable:  true,
		IsFinal:      cmd.changeProcessor.IsFinal,
	}
	fileOutputData, err := filestore.GetFileStore().WriteFile(allocationObj.ID, fileInputData, origfile, connectionObj.ConnectionID)
	if err != nil {
		return result, common.NewError("upload_error", "Failed to upload the file. "+err.Error())
	}

	result.Filename = cmd.changeProcessor.Filename
	result.Hash = fileOutputData.ContentHash
	result.MerkleRoot = fileOutputData.MerkleRoot
	result.Size = fileOutputData.Size

	allocationSize := connectionObj.Size

	// only update connection size when the chunk is uploaded by first time.
	if !fileOutputData.ChunkUploaded {
		allocationSize += fileOutputData.Size
	}

	if allocationSize > config.Configuration.MaxFileSize {
		return result, common.NewError("file_size_limit_exceeded", "Size for the given file is larger than the max limit")
	}

	if allocationObj.BlobberSizeUsed+allocationSize > allocationObj.BlobberSize {
		return result, common.NewError("max_allocation_size", "Max size reached for the allocation with this blobber")
	}

	if len(cmd.changeProcessor.Hash) > 0 && cmd.changeProcessor.Hash != fileOutputData.ContentHash {
		return result, common.NewError("content_hash_mismatch", "Content hash provided in the meta data does not match the file content")
	}

	//push leaf to merkle hasher for computing, save state in db

	//cmd.changeProcessor.TrustedConentHasher.Write(origfile.Read(p []byte))

	// err = cmd.changeProcessor.MerkleHasher.Push(cmd.changeProcessor.Hash, cmd.changeProcessor.ChunkIndex)
	// if errors.Is(err, util.ErrLeafNoSequenced) {

	// 	return result, common.NewError("invalid_chunk_index", "Next chunk index should be "+strconv.Itoa(cmd.changeProcessor.MerkleHasher.Count)+" not "+strconv.Itoa(cmd.changeProcessor.ChunkIndex))
	// }

	cmd.changeProcessor.Hash = fileOutputData.ContentHash

	if cmd.changeProcessor.IsFinal {

		//cmd.changeProcessor.ActualHash = cmd.changeProcessor.MerkleHasher.GetMerkleRoot()
		//cmd.changeProcessor.Hash = fileOutputData.ContentHash
	}

	cmd.changeProcessor.AllocationID = allocationObj.ID
	cmd.changeProcessor.Size = allocationSize

	cmd.allocationChange = &allocation.AllocationChange{}
	cmd.allocationChange.ConnectionID = connectionObj.ConnectionID
	cmd.allocationChange.Size = allocationSize
	cmd.allocationChange.Operation = allocation.RESUME_OPERATION

	connectionObj.Size = allocationSize

	return result, nil
}

// ProcessThumbnail flush thumbnail file to FileStorage if it has.
func (cmd *ResumeFileCommand) ProcessThumbnail(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) error {

	thumbfile, thumbHeader, _ := req.FormFile("uploadThumbnailFile")

	if thumbHeader != nil {

		defer thumbfile.Close()

		thumbInputData := &filestore.FileInputData{Name: thumbHeader.Filename, Path: cmd.changeProcessor.Path}
		thumbOutputData, err := filestore.GetFileStore().WriteFile(allocationObj.ID, thumbInputData, thumbfile, connectionObj.ConnectionID)
		if err != nil {
			return common.NewError("upload_error", "Failed to upload the thumbnail. "+err.Error())
		}
		if cmd.changeProcessor.ThumbnailHash != thumbOutputData.ContentHash {
			return common.NewError("content_hash_mismatch", "Content hash provided in the meta data does not match the thumbnail content")
		}

		cmd.changeProcessor.ThumbnailHash = thumbOutputData.ContentHash
		cmd.changeProcessor.ThumbnailSize = thumbOutputData.Size
		cmd.changeProcessor.ThumbnailFilename = thumbInputData.Name
	}

	return nil
}

func (cmd *ResumeFileCommand) reloadChange(connectionObj *allocation.AllocationChangeCollector) {
	for _, c := range connectionObj.Changes {
		if c.Operation == allocation.RESUME_OPERATION {

			dbChangeProcessor := &allocation.ResumeFileChange{}

			dbChangeProcessor.Unmarshal(c.Input)

			cmd.changeProcessor.Size = dbChangeProcessor.Size
			return
		}
	}
}

// UpdateChange replace ResumeFileChange in db
func (cmd *ResumeFileCommand) UpdateChange(ctx context.Context, connectionObj *allocation.AllocationChangeCollector) error {
	for _, c := range connectionObj.Changes {
		if c.Operation == allocation.RESUME_OPERATION {
			c.Size = connectionObj.Size
			c.Input, _ = cmd.changeProcessor.Marshal()

			c.ModelWithTS.UpdatedAt = time.Now()
			err := connectionObj.Save(ctx)
			if err != nil {
				return err
			}

			return c.Save(ctx)
		}
	}

	//NOT FOUND
	connectionObj.AddChange(cmd.allocationChange, cmd.changeProcessor)

	return connectionObj.Save(ctx)
}
