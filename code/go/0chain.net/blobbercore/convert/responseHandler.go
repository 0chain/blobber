package convert

import (
	"context"
	"encoding/json"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"

	stats2 "github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"

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

func CommitWriteResponseHandler(r interface{}) *blobbergrpc.CommitResponse {
	if r == nil {
		return nil
	}

	httpResp, _ := r.(*blobberHTTP.CommitResult)

	return &blobbergrpc.CommitResponse{
		AllocationRoot: httpResp.AllocationRoot,
		WriteMarker:    WriteMarkerToWriteMarkerGRPC(httpResp.WriteMarker),
		ErrorMessage:   httpResp.ErrorMessage,
		Success:        httpResp.Success,
	}
}

func GetCalculateHashResponseHandler(r interface{}) *blobbergrpc.CalculateHashResponse {
	httpResp, _ := r.(map[string]interface{})
	msg, _ := httpResp["msg"].(string)

	return &blobbergrpc.CalculateHashResponse{Message: msg}
}

func GetCommitMetaTxnHandlerResponse(r interface{}) *blobbergrpc.CommitMetaTxnResponse {
	msg, _ := r.(struct {
		Msg string `json:"msg"`
	})

	return &blobbergrpc.CommitMetaTxnResponse{Message: msg.Msg}
}

func CollaboratorResponse(r interface{}) *blobbergrpc.CollaboratorResponse {
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

func UpdateObjectAttributesResponseCreator(r interface{}) *blobbergrpc.UpdateObjectAttributesResponse {
	if r != nil {
		return nil
	}

	httpResp, _ := r.(*reference.Attributes)
	return &blobbergrpc.UpdateObjectAttributesResponse{WhoPaysForReads: int64(httpResp.WhoPaysForReads)}
}

func CopyObjectResponseCreator(r interface{}) *blobbergrpc.CopyObjectResponse {
	if r == nil {
		return nil
	}

	httpResp, _ := r.(*blobberHTTP.UploadResult)
	return &blobbergrpc.CopyObjectResponse{
		Filename:     httpResp.Filename,
		Size:         httpResp.Size,
		ContentHash:  httpResp.Hash,
		MerkleRoot:   httpResp.MerkleRoot,
		UploadLength: httpResp.UploadLength,
		UploadOffset: httpResp.UploadOffset,
	}
}

func GetAllocationResponseHandler(resp *blobbergrpc.GetAllocationResponse) *allocation.Allocation {
	return GRPCAllocationToAllocation(resp.Allocation)
}

func GetFileMetaDataResponseHandler(resp *blobbergrpc.GetFileMetaDataResponse) map[string]interface{} {
	var collaborators []reference.Collaborator
	for _, c := range resp.Collaborators {
		collaborators = append(collaborators, *GRPCCollaboratorToCollaborator(c))
	}

	result := FileRefGRPCToFileRef(resp.MetaData).GetListingData(context.Background())
	result["collaborators"] = collaborators
	return result
}

func GetFileStatsResponseHandler(resp *blobbergrpc.GetFileStatsResponse) map[string]interface{} {
	ctx := context.Background()
	result := FileRefGRPCToFileRef(resp.MetaData).GetListingData(ctx)

	statsMap := make(map[string]interface{})
	statsBytes, _ := json.Marshal(FileStatsGRPCToFileStats(resp.Stats))
	_ = json.Unmarshal(statsBytes, &statsMap)

	for k, v := range statsMap {
		result[k] = v
	}

	return result
}

func ListEntitesResponseHandler(resp *blobbergrpc.ListEntitiesResponse) *blobberHTTP.ListResult {
	ctx := context.Background()
	var entities []map[string]interface{}
	for i := range resp.Entities {
		entities = append(entities, FileRefGRPCToFileRef(resp.Entities[i]).GetListingData(ctx))
	}

	return &blobberHTTP.ListResult{
		AllocationRoot: resp.AllocationRoot,
		Meta:           FileRefGRPCToFileRef(resp.MetaData).GetListingData(ctx),
		Entities:       entities,
	}
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

func RenameObjectResponseCreator(r interface{}) *blobbergrpc.RenameObjectResponse {
	if r == nil {
		return nil
	}

	httpResp, _ := r.(*blobberHTTP.UploadResult)
	return &blobbergrpc.RenameObjectResponse{
		Filename:     httpResp.Filename,
		Size:         httpResp.Size,
		ContentHash:  httpResp.Hash,
		MerkleRoot:   httpResp.MerkleRoot,
		UploadLength: httpResp.UploadLength,
		UploadOffset: httpResp.UploadOffset,
	}
}