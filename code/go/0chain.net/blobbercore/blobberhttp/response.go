package blobberhttp

import (
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

type UploadResult struct {
	Filename   string `json:"filename"`
	Size       int64  `json:"size"`
	Hash       string `json:"content_hash"`
	MerkleRoot string `json:"merkle_root"`

	// UploadLength indicates the size of the entire upload in bytes. The value MUST be a non-negative integer.
	UploadLength int64 `json:"upload_length"`
	// Upload-Offset indicates a byte offset within a resource. The value MUST be a non-negative integer.
	UploadOffset int64 `json:"upload_offset"`
}

type CommitResult struct {
	AllocationRoot string                         `json:"allocation_root"`
	WriteMarker    *writemarker.WriteMarker       `json:"write_marker"`
	Success        bool                           `json:"success"`
	ErrorMessage   string                         `json:"error_msg,omitempty"`
	Changes        []*allocation.AllocationChange `json:"-"`
	//Result         []*UploadResult         `json:"result"`
}

type ReferencePathResult struct {
	*reference.ReferencePath
	LatestWM *writemarker.WriteMarker `json:"latest_write_marker"`
}

type RefResult struct {
	TotalPages int                       `json:"total_pages"`
	OffsetPath string                    `json:"offset_path,omitempty"` //used for pagination; index for path is created in database
	OffsetDate string                    `json:"offset_date,omitempty"` //used for pagination; idex for updated_at is created in database
	Refs       *[]reference.PaginatedRef `json:"refs"`
	LatestWM   *writemarker.WriteMarker  `json:"latest_write_marker"`
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
	Success      bool   `json:"success"`
	Data         []byte `json:"data"`
	AllocationID string `json:"-"`
	Path         string `json:"-"`
}

type UpdateObjectAttributesResponse struct {
	WhoPaysForReads common.WhoPays `json:"who_pays_for_reads,omitempty"`
}
