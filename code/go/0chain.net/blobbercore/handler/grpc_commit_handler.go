package handler

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"strings"

	blobbergrpc "github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/convert"
)

func (b *blobberGRPCService) Commit(ctx context.Context, req *blobbergrpc.CommitRequest) (*blobbergrpc.CommitResponse, error) {
	body := bytes.NewBuffer([]byte{})
	writer := multipart.NewWriter(body)
	err := writer.WriteField("write_marker", req.WriteMarker)
	if err != nil {
		return nil, err
	}
	err = writer.WriteField("connection_id", req.ConnectionId)
	if err != nil {
		return nil, err
	}
	writer.Close()

	r, err := http.NewRequest("POST", "", strings.NewReader(body.String()))
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, GetGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := CommitHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.CommitWriteResponseCreator(resp), nil
}
