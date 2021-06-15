package convert

import (
	"context"
	"encoding/json"

	stats2 "github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberHTTP"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
)

func GetAllocationResponseCreator(resp interface{}) *blobbergrpc.GetAllocationResponse {
	alloc, _ := resp.(*allocation.Allocation)
	return &blobbergrpc.GetAllocationResponse{Allocation: AllocationToGRPCAllocation(alloc)}
}

func GetFileMetaDataResponseCreator(httpResp interface{}) *blobbergrpc.GetFileMetaDataResponse {
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
	httpResp, _ := r.(*blobberHTTP.ListResult)

	var resp blobbergrpc.ListEntitiesResponse
	for i := range httpResp.Entities {
		resp.Entities = append(resp.Entities, FileRefToFileRefGRPC(reference.ListingDataToRef(httpResp.Entities[i])))
	}

	resp.MetaData = FileRefToFileRefGRPC(reference.ListingDataToRef(httpResp.Meta))
	resp.AllocationRoot = httpResp.AllocationRoot
	return &resp
}

func GetReferencePathResponseHandler(getReferencePathResponse *blobbergrpc.GetReferencePathResponse) *blobberHTTP.ReferencePathResult {
	var recursionCount int
	return &blobberHTTP.ReferencePathResult{
		ReferencePath: ReferencePathGRPCToReferencePath(&recursionCount, getReferencePathResponse.ReferencePath),
		LatestWM:      WriteMarkerGRPCToWriteMarker(getReferencePathResponse.LatestWM),
	}
}

func GetObjectPathResponseHandler(getObjectPathResponse *blobbergrpc.GetObjectPathResponse) *blobberHTTP.ObjectPathResult {
	ctx := context.Background()
	path := FileRefGRPCToFileRef(getObjectPathResponse.ObjectPath.Path).GetListingData(ctx)
	var pathList []map[string]interface{}
	for _, pl := range getObjectPathResponse.ObjectPath.PathList {
		pathList = append(pathList, FileRefGRPCToFileRef(pl).GetListingData(ctx))
	}
	path["list"] = pathList

	return &blobberHTTP.ObjectPathResult{
		ObjectPath: &reference.ObjectPath{
			RootHash:     getObjectPathResponse.ObjectPath.RootHash,
			Meta:         FileRefGRPCToFileRef(getObjectPathResponse.ObjectPath.Meta).GetListingData(ctx),
			Path:         path,
			FileBlockNum: getObjectPathResponse.ObjectPath.FileBlockNum,
		},
		LatestWM: WriteMarkerGRPCToWriteMarker(getObjectPathResponse.LatestWriteMarker),
	}
}

func GetObjectTreeResponseHandler(getObjectTreeResponse *blobbergrpc.GetObjectTreeResponse) *blobberHTTP.ReferencePathResult {
	var recursionCount int
	return &blobberHTTP.ReferencePathResult{
		ReferencePath: ReferencePathGRPCToReferencePath(&recursionCount, getObjectTreeResponse.ReferencePath),
		LatestWM:      WriteMarkerGRPCToWriteMarker(getObjectTreeResponse.LatestWM),
	}
}

func CommitWriteResponseHandler(resp *blobbergrpc.CommitResponse) *blobberHTTP.CommitResult {
	return &blobberHTTP.CommitResult{
		AllocationRoot: resp.AllocationRoot,
		WriteMarker:    WriteMarkerGRPCToWriteMarker(resp.WriteMarker),
		Success:        resp.Success,
		ErrorMessage:   resp.ErrorMessage,
	}
}

func GetCalculateHashResponseHandler(response *blobbergrpc.CalculateHashResponse) interface{} {
	result := make(map[string]interface{})
	if msg := response.GetMessage(); msg != "" {
		result["msg"] = msg
	}

	return result
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
