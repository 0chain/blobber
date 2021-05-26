package convert

import (
	"context"
	"encoding/json"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberHTTP"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
)

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

func DownloadFileResponseHandler(downloadFileResponse *blobbergrpc.DownloadFileResponse) *blobberHTTP.DownloadResponse {
	return &blobberHTTP.DownloadResponse{
		Success:      downloadFileResponse.Success,
		Data:         downloadFileResponse.Data,
		AllocationID: downloadFileResponse.AllocationId,
		Path:         downloadFileResponse.Path,
		LatestRM:     ReadMakerGRPCToReadMaker(downloadFileResponse.LatestRm),
	}
}
