package convert

import (
	"encoding/json"

	stats2 "github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberHTTP"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
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

func GetFileStatsResponseCreator(r interface{}) *blobbergrpc.GetFileStatsResponse {
	if r == nil {
		return nil
	}

	httpResp, _ := r.(map[string]interface{})

	var resp blobbergrpc.GetFileStatsResponse
	resp.MetaData = FileRefToFileRefGRPC(reference.ListingDataToRef(httpResp))

	respRaw, _ := json.Marshal(httpResp)
	var stats stats2.FileStats
	_ = json.Unmarshal(respRaw, &stats)
	resp.Stats = FileStatsToFileStatsGRPC(&stats)

	return &resp
}

func ListEntitesResponseCreator(r interface{}) *blobbergrpc.ListEntitiesResponse {
	if r == nil {
		return nil
	}

	httpResp, _ := r.(*blobberHTTP.ListResult)

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

	httpResp, _ := r.(*blobberHTTP.ReferencePathResult)
	var resp blobbergrpc.GetReferencePathResponse

	var recursionCount int
	resp.LatestWM = WriteMarkerToWriteMarkerGRPC(httpResp.LatestWM)
	resp.ReferencePath = ReferencePathToReferencePathGRPC(&recursionCount, httpResp.ReferencePath)
	return &resp
}

func GetObjectTreeResponseCreator(r interface{}) *blobbergrpc.GetObjectTreeResponse {
	if r == nil {
		return nil
	}

	httpResp, _ := r.(*blobberHTTP.ReferencePathResult)
	var resp blobbergrpc.GetObjectTreeResponse

	var recursionCount int
	resp.LatestWM = WriteMarkerToWriteMarkerGRPC(httpResp.LatestWM)
	resp.ReferencePath = ReferencePathToReferencePathGRPC(&recursionCount, httpResp.ReferencePath)
	return &resp
}

func GetObjectPathResponseCreator(r interface{}) *blobbergrpc.GetObjectPathResponse {
	if r == nil {
		return nil
	}

	httpResp, _ := r.(*blobberHTTP.ObjectPathResult)
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

func CommitWriteResponseHandler(resp *blobbergrpc.CommitResponse) *blobberHTTP.CommitResult {
	return &blobberHTTP.CommitResult{
		AllocationRoot: resp.AllocationRoot,
		WriteMarker:    WriteMarkerGRPCToWriteMarker(resp.WriteMarker),
		Success:        resp.Success,
		ErrorMessage:   resp.ErrorMessage,
	}
}

func GetCalculateHashResponseHandler(r interface{}) *blobbergrpc.CalculateHashResponse {
	httpResp, _ := r.(map[string]interface{})
	msg, _ := httpResp["msg"].(string)

	return &blobbergrpc.CalculateHashResponse{Message: msg}
}

func GetCommitMetaTxnHandlerResponse(response *blobbergrpc.CommitMetaTxnResponse) interface{} {
	msg := response.GetMessage()
	if msg == "" {
		return nil
	}

	result := struct {
		Msg string `json:"msg"`
	}{
		Msg: msg,
	}

	return result
}

func CollaboratorResponse(response *blobbergrpc.CollaboratorResponse) interface{} {
	if msg := response.GetMessage(); msg != "" {
		return struct {
			Msg string `json:"msg"`
		}{Msg: msg}
	}

	if collaborators := response.GetCollaborators(); collaborators != nil {
		collabs := make([]reference.Collaborator, 0, len(collaborators))
		for _, c := range collaborators {
			collabs = append(collabs, *GRPCCollaboratorToCollaborator(c))
		}

		return collabs
	}

	return nil
}

func UpdateObjectAttributesResponseHandler(updateAttributesResponse *blobbergrpc.UpdateObjectAttributesResponse) *blobberHTTP.UpdateObjectAttributesResponse {
	return &blobberHTTP.UpdateObjectAttributesResponse{
		WhoPaysForReads: common.WhoPays(updateAttributesResponse.WhoPaysForReads),
	}
}

func CopyObjectResponseHandler(copyObjectResponse *blobbergrpc.CopyObjectResponse) *blobberHTTP.UploadResult {
	return &blobberHTTP.UploadResult{
		Filename:     copyObjectResponse.Filename,
		Size:         copyObjectResponse.Size,
		Hash:         copyObjectResponse.ContentHash,
		MerkleRoot:   copyObjectResponse.MerkleRoot,
		UploadLength: copyObjectResponse.UploadLength,
		UploadOffset: copyObjectResponse.UploadOffset,
	}
}
