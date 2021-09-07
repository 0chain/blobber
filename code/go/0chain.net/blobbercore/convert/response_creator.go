package convert

import (
	"encoding/json"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	stats2 "github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func GetAllocationResponseCreator(resp interface{}) *blobbergrpc.GetAllocationResponse {
	if resp == nil {
		return nil
	}

	alloc, _ := resp.(*allocation.Allocation)
	return &blobbergrpc.GetAllocationResponse{Allocation: AllocationToGRPCAllocation(alloc)}
}

func GetFileMetaDataResponseCreator(httpResp interface{}) *blobbergrpc.GetFileMetaDataResponse {
	if httpResp == nil {
		return nil
	}

	r, _ := httpResp.(map[string]interface{})

	var resp blobbergrpc.GetFileMetaDataResponse
	collaborators, _ := r["collaborators"].([]reference.Collaborator)
	for _, c := range collaborators {
		resp.Collaborators = append(resp.Collaborators, CollaboratorToGRPCCollaborator(&c))
	}

	resp.MetaData = FileRefToFileRefGRPC(reference.ListingDataToRef(r))
	return &resp
}

func GetFileStatsResponseCreator(r interface{}) (*blobbergrpc.GetFileStatsResponse, error) {
	if r == nil {
		return nil, errors.Wrapf(errors.New("EMPTY REQUEST"), "empty interface passed")
		
	}

	httpResp, _ := r.(map[string]interface{})
	logging.Logger.Info("filestat resposne is", zap.Any("httpResp", httpResp))

	var resp blobbergrpc.GetFileStatsResponse
	resp.MetaData = FileRefToFileRefGRPC(reference.ListingDataToRef(httpResp))

	logging.Logger.Info("filestat response was successfully fetched", zap.Any("metadata", resp.MetaData))
	respRaw, err := json.Marshal(httpResp)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal httpResp")
	}
	var stats stats2.FileStats
	err = json.Unmarshal(respRaw, &stats)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal respRaw into stats")
	}
	resp.Stats = FileStatsToFileStatsGRPC(&stats)

	return &resp, nil
}

func ListEntitesResponseCreator(r interface{}) *blobbergrpc.ListEntitiesResponse {
	if r == nil {
		return nil
	}

	httpResp, _ := r.(*blobberhttp.ListResult)

	var resp blobbergrpc.ListEntitiesResponse
	for i := range httpResp.Entities {
		resp.Entities = append(resp.Entities, FileRefToFileRefGRPC(reference.ListingDataToRef(httpResp.Entities[i])))
	}

	resp.MetaData = FileRefToFileRefGRPC(reference.ListingDataToRef(httpResp.Meta))
	resp.AllocationRoot = httpResp.AllocationRoot
	return &resp
}

func GetReferencePathResponseCreator(r interface{}) *blobbergrpc.GetReferencePathResponse {
	if r == nil {
		return nil
	}

	httpResp, _ := r.(*blobberhttp.ReferencePathResult)
	var resp blobbergrpc.GetReferencePathResponse

	var recursionCount int
	resp.LatestWm = WriteMarkerToWriteMarkerGRPC(httpResp.LatestWM)
	resp.ReferencePath = ReferencePathToReferencePathGRPC(&recursionCount, httpResp.ReferencePath)
	return &resp
}

func GetObjectTreeResponseCreator(r interface{}) *blobbergrpc.GetObjectTreeResponse {
	if r == nil {
		return nil
	}

	httpResp, _ := r.(*blobberhttp.ReferencePathResult)
	var resp blobbergrpc.GetObjectTreeResponse

	var recursionCount int
	resp.LatestWm = WriteMarkerToWriteMarkerGRPC(httpResp.LatestWM)
	resp.ReferencePath = ReferencePathToReferencePathGRPC(&recursionCount, httpResp.ReferencePath)
	return &resp
}

func GetObjectPathResponseCreator(r interface{}) *blobbergrpc.GetObjectPathResponse {
	if r == nil {
		return nil
	}

	httpResp, _ := r.(*blobberhttp.ObjectPathResult)
	var resp blobbergrpc.GetObjectPathResponse

	var pathList []*blobbergrpc.FileRef
	pl, _ := httpResp.Path["list"].([]map[string]interface{})
	for _, v := range pl {
		pathList = append(pathList, FileRefToFileRefGRPC(reference.ListingDataToRef(v)))
	}

	resp.LatestWriteMarker = WriteMarkerToWriteMarkerGRPC(httpResp.LatestWM)
	resp.ObjectPath = &blobbergrpc.ObjectPath{
		RootHash:     httpResp.RootHash,
		Meta:         FileRefToFileRefGRPC(reference.ListingDataToRef(httpResp.Meta)),
		Path:         FileRefToFileRefGRPC(reference.ListingDataToRef(httpResp.Path)),
		PathList:     pathList,
		FileBlockNum: httpResp.FileBlockNum,
	}

	return &resp
}

func CommitWriteResponseCreator(r interface{}) *blobbergrpc.CommitResponse {
	if r == nil {
		return nil
	}

	httpResp, _ := r.(*blobberhttp.CommitResult)

	return &blobbergrpc.CommitResponse{
		AllocationRoot: httpResp.AllocationRoot,
		WriteMarker:    WriteMarkerToWriteMarkerGRPC(httpResp.WriteMarker),
		ErrorMessage:   httpResp.ErrorMessage,
		Success:        httpResp.Success,
	}
}

func CollaboratorResponseCreator(r interface{}) *blobbergrpc.CollaboratorResponse {
	if r == nil {
		return nil
	}

	msg, _ := r.(struct {
		Msg string `json:"msg"`
	})
	var resp blobbergrpc.CollaboratorResponse
	if msg.Msg != "" {
		resp.Message = msg.Msg
		return &resp
	}

	collabs, _ := r.([]reference.Collaborator)
	for _, c := range collabs {
		resp.Collaborators = append(resp.Collaborators, CollaboratorToGRPCCollaborator(&c))
	}

	return &resp
}

func DownloadFileResponseCreator(r interface{}) *blobbergrpc.DownloadFileResponse {
	if r == nil {
		return nil
	}

	switch httpResp := r.(type) {
	case []byte:
		return &blobbergrpc.DownloadFileResponse{
			Data: httpResp,
		}
	case *blobberhttp.DownloadResponse:
		return &blobbergrpc.DownloadFileResponse{
			Success:      httpResp.Success,
			Data:         httpResp.Data,
			AllocationId: httpResp.AllocationID,
			Path:         httpResp.Path,
			LatestRm:     ReadMarkerToReadMarkerGRPC(httpResp.LatestRM),
		}
	}

	return nil
}

func UploadFileResponseCreator(r interface{}) *blobbergrpc.UploadFileResponse {
	if r == nil {
		return nil
	}

	httpResp, _ := r.(*blobberhttp.UploadResult)
	return &blobbergrpc.UploadFileResponse{
		Filename:     httpResp.Filename,
		Size:         httpResp.Size,
		ContentHash:  httpResp.Hash,
		MerkleRoot:   httpResp.MerkleRoot,
		UploadLength: httpResp.UploadLength,
		UploadOffset: httpResp.UploadOffset,
	}
}
