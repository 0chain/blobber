package blobberhttp

import (
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

// swagger:model ConnectionResult
type ConnectionResult struct {
	AllocationRoot string `json:"allocation_root"`
	ConnectionID   string `json:"connection_id"`
}

// swagger:model CommitResult
type CommitResult struct {
	AllocationRoot string                         `json:"allocation_root"`
	WriteMarker    *writemarker.WriteMarkerEntity `json:"write_marker"`
	Success        bool                           `json:"success"`
	ErrorMessage   string                         `json:"error_msg,omitempty"`
	//Result         []*UploadResult         `json:"result"`
}

// swagger:model ReferencePathResult
type ReferencePathResult struct {
	*reference.ReferencePath
	LatestWM *writemarker.WriteMarker `json:"latest_write_marker"`
	Version  string                   `json:"version"`
}

// swagger:model RefResult
type RefResult struct {
	TotalPages int                       `json:"total_pages"`
	OffsetPath string                    `json:"offset_path,omitempty"` //used for pagination; index for path is created in database
	OffsetDate common.Timestamp          `json:"offset_date,omitempty"` //used for pagination; idex for updated_at is created in database
	Refs       *[]reference.PaginatedRef `json:"refs"`
	LatestWM   *writemarker.WriteMarker  `json:"latest_write_marker"`
}

// swagger:model RecentRefResult
type RecentRefResult struct {
	Offset int                       `json:"offset"`
	Refs   []*reference.PaginatedRef `json:"refs"`
}
type ObjectPathResult struct {
	*reference.ObjectPath
	LatestWM *writemarker.WriteMarker `json:"latest_write_marker"`
}

// swagger:model ListResult
type ListResult struct {
	AllocationRoot string                   `json:"allocation_root"`
	Meta           map[string]interface{}   `json:"meta_data"`
	Entities       []map[string]interface{} `json:"list"`
}

// swagger:model DownloadResponse
type DownloadResponse struct {
	Success      bool                   `json:"success"`
	Data         []byte                 `json:"data"`
	AllocationID string                 `json:"-"`
	Path         string                 `json:"-"`
	LatestRM     *readmarker.ReadMarker `json:"latest_rm"`
}

// swagger:model LatestWriteMarkerResult
type LatestWriteMarkerResult struct {
	LatestWM *writemarker.WriteMarker `json:"latest_write_marker"`
	PrevWM   *writemarker.WriteMarker `json:"prev_write_marker"`
	Version  string                   `json:"version"`
}

type LatestVersionMarkerResult struct {
	VersionMarker *writemarker.VersionMarker `json:"version_marker"`
}
