package stats

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"0chain.net/blobbercore/config"
	"0chain.net/blobbercore/datastore"
	"0chain.net/blobbercore/filestore"
	"0chain.net/core/common"

	"0chain.net/core/node"

	. "0chain.net/core/logging"
	"go.uber.org/zap"

	"github.com/jinzhu/gorm/dialects/postgres"
)

const DateTimeFormat = "2006-01-02T15:04:05"

// ReadMarkersStat represents read markers redeeming
// statistics for a blobber or for an allocation.
type ReadMarkersStat struct {
	Redeemed int64 `gorm:"redeemed" json:"redeemed"`
	Pending  int64 `gorm:"pending" json:"pending"`
}

type WriteMarkers struct {
	Size  int64 `gorm:"size" json:"size"`
	Count int64 `gorm:"count" json:"count"`
}

// WriteMarkersStat represents write markers redeeming
// statistic for a blobber or for an allocation.
type WriteMarkersStat struct {
	Accepted  WriteMarkers `gorm:"accepted" json:"accepted"`
	Committed WriteMarkers `gorm:"committed" json:"committed"`
	Failed    WriteMarkers `gorm:"failed" json:"failed"`
}

type Stats struct {
	UsedSize           int64 `json:"used_size"`
	FilesSize          int64 `json:"files_size"`
	ThumbnailsSize     int64 `json:"thumbnails_size"`
	DiskSizeUsed       int64 `json:"disk_size_used"`
	BlockWrites        int64 `json:"num_of_block_writes"`
	NumWrites          int64 `json:"num_of_writes"`
	NumReads           int64 `json:"num_of_reads"`
	TotalChallenges    int64 `json:"total_challenges"`
	OpenChallenges     int64 `json:"num_open_challenges"`
	SuccessChallenges  int64 `json:"num_success_challenges"`
	FailedChallenges   int64 `json:"num_failed_challenges"`
	RedeemedChallenges int64 `json:"num_redeemed_challenges"`
}

var LastMinioScan time.Time

type MinioStats struct {
	CloudFilesSize  int64  `json:"cloud_files_size"`
	CloudTotalFiles int    `json:"cloud_total_files"`
	LastMinioScan   string `json:"last_minio_scan"`
}

type Duration int64

func (d Duration) String() string {
	return (time.Duration(d) * time.Second).String()
}

type BlobberStats struct {
	Stats
	MinioStats
	NumAllocation int64  `json:"num_of_allocations"`
	ClientID      string `json:"-"`
	PublicKey     string `json:"-"`

	// configurations
	Capacity                int64         `json:"capacity"`
	ReadPrice               float64       `json:"read_price"`
	WritePrice              float64       `json:"write_price"`
	MinLockDemand           float64       `json:"min_lock_demand"`
	MaxOfferDuration        time.Duration `json:"max_offer_duration"`
	ChallengeCompletionTime time.Duration `json:"challnge_completion_time"`
	ReadLockTimeout         Duration      `json:"read_lock_timeout"`
	WriteLockTimeout        Duration      `json:"write_lock_timeout"`

	AllocationStats []*AllocationStats `json:"-"`

	// total for all allocations
	ReadMarkers  ReadMarkersStat  `json:"read_markers"`
	WriteMarkers WriteMarkersStat `json:"write_markers"`
}

type AllocationId struct {
	Id string `json:"id"`
}

func LoadBlobberStats(ctx context.Context) *BlobberStats {
	fs := &BlobberStats{}
	fs.loadBasicStats(ctx)
	fs.loadDetailedStats(ctx)
	return fs
}

func (bs *BlobberStats) loadBasicStats(ctx context.Context) {
	bs.AllocationStats = make([]*AllocationStats, 0)
	bs.ClientID = node.Self.ID
	bs.PublicKey = node.Self.PublicKey
	// configurations
	bs.Capacity = config.Configuration.Capacity
	bs.ReadPrice = config.Configuration.ReadPrice
	bs.WritePrice = config.Configuration.WritePrice
	bs.MinLockDemand = config.Configuration.MinLockDemand
	bs.MaxOfferDuration = config.Configuration.MaxOfferDuration
	bs.ChallengeCompletionTime = config.Configuration.ChallengeCompletionTime
	bs.ReadLockTimeout = Duration(config.Configuration.ReadLockTimeout)
	bs.WriteLockTimeout = Duration(config.Configuration.WriteLockTimeout)
	//
	du, err := filestore.GetFileStore().GetTotalDiskSizeUsed()
	if err != nil {
		du = -1
	}
	bs.DiskSizeUsed = du
	bs.loadStats(ctx)
	bs.loadMinioStats(ctx)
}

func (bs *BlobberStats) loadDetailedStats(ctx context.Context) {
	bs.loadAllocationStats(ctx)
	bs.loadChallengeStats(ctx)
	bs.loadAllocationChallengeStats(ctx)

	// load read/write markers stat
	var (
		given = make(map[string]struct{})
		err   error
	)
	for _, as := range bs.AllocationStats {
		if _, ok := given[as.AllocationID]; ok {
			continue
		}
		given[as.AllocationID] = struct{}{}
		as.ReadMarkers, err = loadAllocReadMarkersStat(ctx, as.AllocationID)
		if err != nil {
			Logger.Error("getting read_maker stat",
				zap.String("allocation_id", as.AllocationID),
				zap.Error(err))
		} else {
			bs.ReadMarkers.Pending += as.ReadMarkers.Pending
			bs.ReadMarkers.Redeemed += as.ReadMarkers.Redeemed
		}
		as.WriteMarkers, err = loadAllocWriteMarkerStat(ctx, as.AllocationID)
		if err != nil {
			Logger.Error("getting write_maker stat",
				zap.String("allocation_id", as.AllocationID),
				zap.Error(err))
		} else {
			bs.WriteMarkers.Accepted.Count += as.WriteMarkers.Accepted.Count
			bs.WriteMarkers.Accepted.Size += as.WriteMarkers.Accepted.Size
			bs.WriteMarkers.Committed.Count += as.WriteMarkers.Committed.Count
			bs.WriteMarkers.Committed.Size += as.WriteMarkers.Committed.Size
			bs.WriteMarkers.Failed.Count += as.WriteMarkers.Failed.Count
			bs.WriteMarkers.Failed.Size += as.WriteMarkers.Failed.Size
		}
	}
}

func (bs *BlobberStats) loadStats(ctx context.Context) {
	db := datastore.GetStore().GetTransaction(ctx)
	rows, err := db.Table("reference_objects").Select(
		"SUM(reference_objects.size) as files_size, SUM(reference_objects.thumbnail_size) as thumbnails_size, SUM(file_stats.num_of_block_downloads) as num_of_reads, SUM(reference_objects.num_of_blocks) as num_of_block_writes, COUNT(*) as num_of_writes",
	).Joins("inner join file_stats on reference_objects.id = file_stats.ref_id where reference_objects.type = 'f' and reference_objects.deleted_at IS NULL").Rows()

	if err != nil {
		Logger.Error("Error in getting the blobber stats", zap.Error(err))
	}
	for rows.Next() {
		err = rows.Scan(&bs.FilesSize, &bs.ThumbnailsSize, &bs.NumReads, &bs.BlockWrites, &bs.NumWrites)
		if err != nil {
			Logger.Error("Error in scanning record for blobber stats", zap.Error(err))
		}
		break
	}
	rows.Close()
	bs.UsedSize = bs.FilesSize + bs.ThumbnailsSize
	db.Table("allocations").Count(&bs.NumAllocation)
}

func (bs *BlobberStats) loadMinioStats(ctx context.Context) {
	db := datastore.GetStore().GetTransaction(ctx)
	rows, err := db.Table("reference_objects").
		Select("SUM(size) as cloud_files_size,COUNT(*) as cloud_total_files").
		Where("on_cloud = 'TRUE' and type = 'f' and deleted_at IS NULL").Rows()
	if err != nil {
		Logger.Error("Error in getting the minio stats", zap.Error(err))
	}
	for rows.Next() {
		err = rows.Scan(&bs.CloudFilesSize, &bs.CloudTotalFiles)
		if err != nil {
			Logger.Error("Error in scanning record for minio stats", zap.Error(err))
		}
		break
	}
	rows.Close()
	bs.LastMinioScan = LastMinioScan.Format(DateTimeFormat)
}

func (bs *BlobberStats) loadAllocationStats(ctx context.Context) {
	bs.AllocationStats = make([]*AllocationStats, 0)
	db := datastore.GetStore().GetTransaction(ctx)
	rows, err := db.Table("reference_objects").
		Select(`
            reference_objects.allocation_id,
            SUM(reference_objects.size) as files_size,
            SUM(reference_objects.thumbnail_size) as thumbnails_size,
            SUM(file_stats.num_of_block_downloads) as num_of_reads,
            SUM(reference_objects.num_of_blocks) as num_of_block_writes,
            COUNT(*) as num_of_writes,
            allocations.expiration_date AS expiration_date`).
		Joins(`INNER JOIN file_stats
            ON reference_objects.id = file_stats.ref_id`).
		Joins(`
            INNER JOIN allocations
            ON allocations.id = reference_objects.allocation_id`).
		Where(`reference_objects.type = 'f'
            AND reference_objects.deleted_at IS NULL`).
		Group(`reference_objects.allocation_id, allocations.expiration_date`).
		Rows()

	if err != nil {
		Logger.Error("Error in getting the allocation stats", zap.Error(err))
	}

	for rows.Next() {
		as := &AllocationStats{}
		err = rows.Scan(&as.AllocationID, &as.FilesSize, &as.ThumbnailsSize,
			&as.NumReads, &as.BlockWrites, &as.NumWrites, &as.Expiration)
		if err != nil {
			Logger.Error("Error in scanning record for blobber stats", zap.Error(err))
		}
		as.UsedSize = as.FilesSize + as.ThumbnailsSize
		as.loadAllocationDiskUsageStats()
		bs.AllocationStats = append(bs.AllocationStats, as)
	}
	rows.Close()

}

func (bs *BlobberStats) loadChallengeStats(ctx context.Context) {
	db := datastore.GetStore().GetTransaction(ctx)
	rows, err := db.Table("challenges").
		Select(`COUNT(*) as total_challenges,
		challenges.status,
		challenges.result`).
		Group(`challenges.status, challenges.result`).
		Rows()
	if err != nil {
		Logger.Error("Error in getting the blobber challenge stats", zap.Error(err))
	}

	for rows.Next() {
		total := int64(0)
		status := 0
		result := 0

		err = rows.Scan(&total, &status, &result)
		if err != nil {
			Logger.Error("Error in scanning record for blobber stats", zap.Error(err))
		}
		bs.TotalChallenges += total
		if status == 3 {
			bs.RedeemedChallenges += total
		} else {
			bs.OpenChallenges += total
		}

		if result == 1 {
			bs.SuccessChallenges += total
		} else if result == 2 {
			bs.FailedChallenges += total
		}
	}
	rows.Close()
}

func (bs *BlobberStats) loadAllocationChallengeStats(ctx context.Context) {
	db := datastore.GetStore().GetTransaction(ctx)
	rows, err := db.Table("challenges").Select(`
        challenges.allocation_id,
        COUNT(*) as total_challenges,
        challenges.status,
        challenges.result`).
		Group(`challenges.allocation_id, challenges.status, challenges.result`).
		Rows()
	if err != nil {
		Logger.Error("Error in getting the allocation challenge stats",
			zap.Error(err))
	}

	allocationStatsMap := make(map[string]*AllocationStats)

	for _, as := range bs.AllocationStats {
		allocationStatsMap[as.AllocationID] = as
	}

	for rows.Next() {
		total := int64(0)
		status := 0
		result := 0
		allocationID := ""
		err = rows.Scan(&allocationID, &total, &status, &result)
		if err != nil {
			Logger.Error("Error in scanning record for blobber stats", zap.Error(err))
		}
		as := allocationStatsMap[allocationID]
		if as == nil {
			continue
		}
		as.TotalChallenges += total
		if status == 3 {
			as.RedeemedChallenges += total
		} else {
			as.OpenChallenges += total
		}

		if result == 1 {
			as.SuccessChallenges += total
		} else if result == 2 {
			as.FailedChallenges += total
		}
	}
	rows.Close()
}

func loadAllocationList(ctx context.Context) (interface{}, error) {
	allocations := make([]AllocationId, 0)
	db := datastore.GetStore().GetTransaction(ctx)
	rows, err := db.Table("reference_objects").Select(
		"reference_objects.allocation_id").Group("reference_objects.allocation_id").Rows()

	if err != nil {
		Logger.Error("Error in getting the allocation list", zap.Error(err))
		return nil, common.NewError("get_allocations_list_failed", "Failed to get allocation list from DB")
	}

	for rows.Next() {
		var allocationId AllocationId
		err = rows.Scan(&allocationId.Id)
		if err != nil {
			Logger.Error("Error in scanning record for blobber allocations", zap.Error(err))
		}
		allocations = append(allocations, allocationId)
	}
	rows.Close()
	return allocations, nil
}

type ReadMarkerEntity struct {
	ReadCounter          int64          `gorm:"column:counter" json:"counter"`
	LatestRedeemedRMBlob postgres.Jsonb `gorm:"column:latest_redeemed_rm"`
	RedeemRequired       bool           `gorm:"column:redeem_required"`
}

func loadAllocReadMarkersStat(ctx context.Context, allocationID string) (
	rms *ReadMarkersStat, err error) {

	var (
		db  = datastore.GetStore().GetTransaction(ctx)
		rme ReadMarkerEntity
	)
	err = db.Table("read_markers").
		Select("counter, latest_redeemed_rm, redeem_required").
		Where("allocation_id = ?", allocationID).
		Limit(1).
		Row().
		Scan(&rme.ReadCounter, &rme.LatestRedeemedRMBlob, &rme.RedeemRequired)

	if err != nil && err != sql.ErrNoRows {
		return
	}

	if err == sql.ErrNoRows {
		return &ReadMarkersStat{}, nil // empty
	}

	var prev, current = new(ReadMarkerEntity), &rme
	if len(rme.LatestRedeemedRMBlob.RawMessage) > 0 {
		err = json.Unmarshal([]byte(rme.LatestRedeemedRMBlob.RawMessage), prev)
		if err != nil {
			return
		}
	}

	rms = new(ReadMarkersStat)
	if current.RedeemRequired {
		rms.Pending = current.ReadCounter - prev.ReadCounter // pending
		rms.Redeemed = prev.ReadCounter                      // already redeemed
	} else {
		rms.Redeemed = current.ReadCounter // already redeemed
	}

	return
}

// copy pasted from writemarker package because of import cycle
type WriteMarkerStatus int

const (
	Accepted  WriteMarkerStatus = iota // 0
	Committed                          // 1
	Failed                             // 2
)

func loadAllocWriteMarkerStat(ctx context.Context, allocationID string) (
	wms *WriteMarkersStat, err error) {

	var (
		db   = datastore.GetStore().GetTransaction(ctx)
		rows *sql.Rows
	)

	rows, err = db.Table("write_markers").
		Select(`status, SUM(size) AS size, COUNT(size) as count`).
		Where("allocation_id = ?", allocationID).
		Group("status").
		Rows()

	if err != nil {
		return
	}

	defer rows.Close()

	type writeMarkerRow struct {
		Status WriteMarkerStatus
		Size   int64
		Count  int64
	}

	wms = new(WriteMarkersStat)

	for rows.Next() {
		var wmr writeMarkerRow
		err = rows.Scan(&wmr.Status, &wmr.Size, &wmr.Count)
		if err != nil {
			return nil, err
		}
		switch wmr.Status {
		case Accepted:
			wms.Accepted.Count += wmr.Count
			wms.Accepted.Size += wmr.Size
		case Committed:
			wms.Committed.Count += wmr.Count
			wms.Committed.Size += wmr.Size
		case Failed:
			wms.Failed.Count += wmr.Count
			wms.Failed.Size += wmr.Size
		}
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return
}
