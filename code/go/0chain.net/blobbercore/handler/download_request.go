package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/errors"
)

// DownloadRequest metedata of download request
type DownloadRequest struct {
	req          *http.Request
	allocationID string
	PathHash     string
	Path         string
	BlockNum     int64
	NumBlocks    int64
	ReadMarker   readmarker.ReadMarker
	AuthToken    string
	RxPay        bool
	DownloadMode string
}

func FromDownloadRequest(allocationID string, req *http.Request) (*DownloadRequest, error) {
	if allocationID == "" {
		return nil, errors.Throw(common.ErrInvalidParameter, "allocationID")
	}

	if req == nil {
		return nil, errors.Throw(common.ErrInvalidParameter, "req")
	}

	dr := &DownloadRequest{
		allocationID: allocationID,
		req:          req,
	}

	err := dr.Parse()
	if err != nil {
		return nil, err
	}

	return dr, nil
}

func (dr *DownloadRequest) Parse() error {
	if dr.req == nil {
		return errors.Throw(common.ErrInvalidParameter, "req")
	}

	pathHash := dr.Get("path_hash")
	path := dr.Get("path")
	if pathHash == "" {
		if path == "" {
			return errors.Throw(common.ErrInvalidParameter, "path")
		}
		pathHash = reference.GetReferenceLookup(dr.allocationID, path)
	}

	dr.PathHash = pathHash
	dr.Path = path

	blockNum := dr.GetInt64("block_num", -1)
	if blockNum <= 0 {
		return errors.Throw(common.ErrInvalidParameter, "block_num")
	}
	dr.BlockNum = blockNum

	numBlocks := dr.GetInt64("num_blocks", 1)
	if numBlocks <= 0 {
		return errors.Throw(common.ErrInvalidParameter, "num_blocks")
	}
	dr.NumBlocks = numBlocks

	readMarker := dr.Get("read_marker")

	if readMarker == "" {
		return errors.Throw(common.ErrInvalidParameter, "read_marker")
	}

	err := json.Unmarshal([]byte(readMarker), &dr.ReadMarker)
	if err != nil {
		return errors.Throw(common.ErrInvalidParameter, "read_marker")
	}

	dr.AuthToken = dr.Get("auth_token")

	dr.RxPay = dr.Get("rx_pay") == "true"
	dr.DownloadMode = dr.Get("content")

	return nil
}

func (dr *DownloadRequest) Get(key string) string {
	if dr.req == nil {
		return ""
	}
	v := dr.req.Header.Get(key)

	if v == "" {
		v = dr.req.FormValue(key)
	}

	return v

}

func (dr *DownloadRequest) GetInt64(key string, defaultValue int64) int64 {
	v := dr.Get(key)
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return defaultValue
	}

	return i
}
