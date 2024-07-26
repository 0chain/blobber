package convert

import (
	"context"
	"encoding/json"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
)

func GetAllocationResponseHandler(resp *blobbergrpc.GetAllocationResponse) *allocation.Allocation {
	return GRPCAllocationToAllocation(resp.Allocation)
}

func GetFileMetaDataResponseHandler(resp *blobbergrpc.GetFileMetaDataResponse) map[string]interface{} {
	result := FileRefGRPCToFileRef(resp.MetaData).GetListingData(context.Background())
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

func ListEntitesResponseHandler(resp *blobbergrpc.ListEntitiesResponse) *blobberhttp.ListResult {
	ctx := context.Background()
	var entities []map[string]interface{}
	for i := range resp.Entities {
		entities = append(entities, FileRefGRPCToFileRef(resp.Entities[i]).GetListingData(ctx))
	}

	return &blobberhttp.ListResult{
		Meta:     FileRefGRPCToFileRef(resp.MetaData).GetListingData(ctx),
		Entities: entities,
	}
}

func GetReferencePathResponseHandler(getReferencePathResponse *blobbergrpc.GetReferencePathResponse) *blobberhttp.ReferencePathResult {
	var recursionCount int
	return &blobberhttp.ReferencePathResult{
		ReferencePath: ReferencePathGRPCToReferencePath(&recursionCount, getReferencePathResponse.ReferencePath),
		LatestWM:      WriteMarkerGRPCToWriteMarker(getReferencePathResponse.LatestWm),
	}
}

func GetObjectPathResponseHandler(getObjectPathResponse *blobbergrpc.GetObjectPathResponse) *blobberhttp.ObjectPathResult {
	ctx := context.Background()
	path := FileRefGRPCToFileRef(getObjectPathResponse.ObjectPath.Path).GetListingData(ctx)
	var pathList []map[string]interface{}
	for _, pl := range getObjectPathResponse.ObjectPath.PathList {
		pathList = append(pathList, FileRefGRPCToFileRef(pl).GetListingData(ctx))
	}
	path["list"] = pathList

	return &blobberhttp.ObjectPathResult{
		ObjectPath: &reference.ObjectPath{
			RootHash:     getObjectPathResponse.ObjectPath.RootHash,
			Meta:         FileRefGRPCToFileRef(getObjectPathResponse.ObjectPath.Meta).GetListingData(ctx),
			Path:         path,
			FileBlockNum: getObjectPathResponse.ObjectPath.FileBlockNum,
		},
		LatestWM: WriteMarkerGRPCToWriteMarker(getObjectPathResponse.LatestWriteMarker),
	}
}

func GetObjectTreeResponseHandler(getObjectTreeResponse *blobbergrpc.GetObjectTreeResponse) *blobberhttp.ReferencePathResult {
	var recursionCount int
	return &blobberhttp.ReferencePathResult{
		ReferencePath: ReferencePathGRPCToReferencePath(&recursionCount, getObjectTreeResponse.ReferencePath),
		LatestWM:      WriteMarkerGRPCToWriteMarker(getObjectTreeResponse.LatestWm),
	}
}

func CommitWriteResponseHandler(resp *blobbergrpc.CommitResponse) *blobberhttp.CommitResult {
	return &blobberhttp.CommitResult{
		AllocationRoot: resp.AllocationRoot,
		// WriteMarker:    WriteMarkerGRPCToWriteMarker(resp.WriteMarker),
		Success:      resp.Success,
		ErrorMessage: resp.ErrorMessage,
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

func CopyObjectResponseHandler(copyObjectResponse *blobbergrpc.CopyObjectResponse) *allocation.UploadResult {
	return &allocation.UploadResult{
		Filename:     copyObjectResponse.Filename,
		Size:         copyObjectResponse.Size,
		UploadLength: copyObjectResponse.UploadLength,
		UploadOffset: copyObjectResponse.UploadOffset,
	}
}
