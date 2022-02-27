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

// DownloadRequestHeader metedata of download request
type DownloadRequestHeader struct {
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

func FromDownloadRequest(allocationID string, req *http.Request) (*DownloadRequestHeader, error) {
	if allocationID == "" {
		return nil, errors.Throw(common.ErrInvalidParameter, "allocationID")
	}

	if req == nil {
		return nil, errors.Throw(common.ErrInvalidParameter, "req")
	}

	dr := &DownloadRequestHeader{
		allocationID: allocationID,
		req:          req,
	}

	err := dr.Parse()
	if err != nil {
		return nil, err
	}

	return dr, nil
}

func (dr *DownloadRequestHeader) Parse() error {
	if dr.req == nil {
		return errors.Throw(common.ErrInvalidParameter, "req")
	}

	pathHash := dr.Get("X-Path-Hash")
	path := dr.Get("X-Path")
	if pathHash == "" {
		if path == "" {
			return errors.Throw(common.ErrInvalidParameter, "X-Path")
		}
		pathHash = reference.GetReferenceLookup(dr.allocationID, path)
	}

	dr.PathHash = pathHash
	dr.Path = path

	blockNum := dr.GetInt64("X-Block-Num", -1)
	if blockNum <= 0 {
		return errors.Throw(common.ErrInvalidParameter, "X-Block-Num")
	}
	dr.BlockNum = blockNum

	numBlocks := dr.GetInt64("X-Num-Blocks", 1)
	if numBlocks <= 0 {
		return errors.Throw(common.ErrInvalidParameter, "X-Num-Blocks")
	}
	dr.NumBlocks = numBlocks

	readMarker := dr.Get("X-Read-Marker")

	if readMarker == "" {
		return errors.Throw(common.ErrInvalidParameter, "X-Read-Marker")
	}

	err := json.Unmarshal([]byte(readMarker), &dr.ReadMarker)
	if err != nil {
		return errors.Throw(common.ErrInvalidParameter, "X-Read-Marker")
	}

	dr.AuthToken = dr.Get("X-Auth-Token")

	dr.RxPay = dr.Get("X-Rxpay") == "true"
	dr.DownloadMode = dr.Get("X-Mode")

	return nil
}

func (dr *DownloadRequestHeader) Get(key string) string {
	if dr.req == nil {
		return ""
	}
	return dr.req.Header.Get(key)
}

func (dr *DownloadRequestHeader) GetInt64(key string, defaultValue int64) int64 {
	v := dr.Get(key)
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return defaultValue
	}

	return i
}
