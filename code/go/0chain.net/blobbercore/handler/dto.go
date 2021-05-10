package handler

import (
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
)

type UploadResult struct {
	Filename   string `json:"filename"`
	Size       int64  `json:"size"`
	Hash       string `json:"content_hash"`
	MerkleRoot string `json:"merkle_root"`
}

type CommitResult struct {
	AllocationRoot string                         `json:"allocation_root"`
	WriteMarker    *writemarker.WriteMarker       `json:"write_marker"`
	Success        bool                           `json:"success"`
	ErrorMessage   string                         `json:"error_msg,omitempty"`
	Changes        []*allocation.AllocationChange `json:"-"`
	//Result         []*UploadResult         `json:"result"`
}

type ReferencePath struct {
	Meta map[string]interface{} `json:"meta_data"`
	List []*ReferencePath       `json:"list,omitempty"`
	ref  *reference.Ref
}

type ReferencePathResult struct {
	*ReferencePath
	LatestWM *writemarker.WriteMarker `json:"latest_write_marker"`
}

type ObjectPathResult struct {
	*reference.ObjectPath
	LatestWM *writemarker.WriteMarker `json:"latest_write_marker"`
}

type ListResult struct {
	AllocationRoot string                   `json:"allocation_root"`
	Meta           map[string]interface{}   `json:"meta_data"`
	Entities       []map[string]interface{} `json:"list"`
}

type DownloadResponse struct {
	Success      bool                   `json:"success"`
	Data         []byte                 `json:"data"`
	AllocationID string                 `json:"-"`
	Path         string                 `json:"-"`
	LatestRM     *readmarker.ReadMarker `json:"latest_rm"`
}
