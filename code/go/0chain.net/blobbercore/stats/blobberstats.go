package stats

import (
	"context"
	"database/sql"
	"encoding/json"
	"runtime"
	"time"

	"go.uber.org/zap"
	"gorm.io/datatypes"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	disk_balancer "github.com/0chain/blobber/code/go/0chain.net/blobbercore/disk-balancer"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
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
	AllocatedSize      int64 `json:"allocated_size"`
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
	CloudFilesSize  int64     `json:"cloud_files_size"`
	CloudTotalFiles int       `json:"cloud_total_files"`
	LastMinioScan   time.Time `json:"last_minio_scan"`
}

type Duration int64

func (d Duration) String() string {
	return (time.Duration(d) * time.Second).String()
}

type BlobberStats struct {
	Stats
	MinioStats
	NumAllocation             int64             `json:"num_of_allocations"`
	ClientID                  string            `json:"-"`
	PublicKey                 string            `json:"-"`
	InfraStats                InfraStats        `json:"-"`
	DBStats                   *DBStats          `json:"-"`
	FailedChallengeList       []ChallengeEntity `json:"-"`
	FailedChallengePagination *Pagination       `json:"failed_challenge_pagination,omitempty"`
	AllocationListPagination  *Pagination       `json:"allocation_list_pagination,omitempty"`

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
	fs.loadInfraStats(ctx)
	fs.loadDBStats()
	fs.loadFailedChallengeList(ctx)
	return fs
}

func (bs *BlobberStats) loadBasicStats(ctx context.Context) {
	bs.AllocationStats = make([]*AllocationStats, 0)
	bs.ClientID = node.Self.ID
	bs.PublicKey = node.Self.PublicKey
	// configurations
	bs.Capacity = disk_balancer.GetDiskSelector().GetCapacity()
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
		bs.AllocatedSize += as.AllocatedSize

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

func (bs *BlobberStats) loadInfraStats(ctx context.Context) {
	healthIn := ctx.Value(HealthDataKey)
	if healthIn == nil {
		return
	}
	health := healthIn.(string)
	memstats := runtime.MemStats{}
	runtime.ReadMemStats(&memstats)
	bs.InfraStats = InfraStats{
		CPUs:               runtime.NumCPU(),
		NumberOfGoroutines: runtime.NumGoroutine(),
		HeapSys:            int64(memstats.HeapSys),
		HeapAlloc:          int64(memstats.HeapAlloc),
		ActiveOnChain:      health,
	}
}

func (bs *BlobberStats) loadDBStats() {
	bs.DBStats = &DBStats{Status: "✗"}

	db := datastore.GetStore().GetDB()
	sqldb, err := db.DB()
	if err != nil {
		return
	}

	dbStats := sqldb.Stats()

	bs.DBStats.Status = "✔"
	bs.DBStats.DBStats = dbStats
}

func (bs *BlobberStats) loadFailedChallengeList(ctx context.Context) {
	fcrdI := ctx.Value(FailedChallengeRequestDataKey)
	if fcrdI == nil {
		return
	}
	fcrd := fcrdI.(RequestData)

	fcs, count, err := getAllFailedChallenges(fcrd.Offset, fcrd.Limit)
	if err != nil {
		Logger.Error("", zap.Any("err", err))
		return
	}
	bs.FailedChallengeList = fcs

	pagination := GeneratePagination(fcrd.Page, fcrd.Limit, fcrd.Offset, count)
	bs.FailedChallengePagination = pagination
}

func (bs *BlobberStats) loadStats(ctx context.Context) {

	const sel = `
	COALESCE (SUM (reference_objects.size), 0) AS files_size,
	COALESCE (SUM (reference_objects.thumbnail_size), 0) AS thumbnails_size,
	COALESCE (SUM (file_stats.num_of_block_downloads), 0) AS num_of_reads,
	COALESCE (SUM (reference_objects.num_of_blocks), 0) AS num_of_block_writes,
	COUNT (*) AS num_of_writes`

	const join = `
	INNER JOIN file_stats ON reference_objects.id = file_stats.ref_id
	WHERE reference_objects.type = 'f'
	AND reference_objects.deleted_at IS NULL`

	var (
		db  = datastore.GetStore().GetTransaction(ctx)
		row *sql.Row
		err error
	)

	row = db.Table("reference_objects").
		Select(sel).
		Joins(join).
		Row()

	err = row.Scan(&bs.FilesSize, &bs.ThumbnailsSize, &bs.NumReads,
		&bs.BlockWrites, &bs.NumWrites)
	if err != nil && err != sql.ErrNoRows {
		Logger.Error("Error in scanning record for blobber stats",
			zap.Error(err))
		return
	}

	bs.UsedSize = bs.FilesSize + bs.ThumbnailsSize
	db.Table("allocations").Count(&bs.NumAllocation)
}

func (bs *BlobberStats) loadMinioStats(ctx context.Context) {

	var (
		db  = datastore.GetStore().GetTransaction(ctx)
		row *sql.Row
		err error
	)

	row = db.Table("reference_objects").
		Select(`
			COALESCE (SUM (size), 0) AS cloud_files_size,
			COUNT (*) AS cloud_total_files`).
		Where("on_cloud = 'TRUE' and type = 'f' and deleted_at IS NULL").
		Row()

	err = row.Scan(&bs.CloudFilesSize, &bs.CloudTotalFiles)
	if err != nil && err != sql.ErrNoRows {
		Logger.Error("Error in scanning record for minio stats",
			zap.Error(err))
		return
	}

	bs.LastMinioScan = LastMinioScan
}

func (bs *BlobberStats) loadAllocationStats(ctx context.Context) {
	bs.AllocationStats = make([]*AllocationStats, 0)

	var (
		db            = datastore.GetStore().GetTransaction(ctx)
		rows          *sql.Rows
		err           error
		requestData   *RequestData
		offset, limit int
	)

	alrdI := ctx.Value(AllocationListRequestDataKey)
	if alrdI == nil {
		offset = 0
		limit = 20
	} else {
		alrd := alrdI.(RequestData)
		requestData = &alrd
		offset = requestData.Offset
		limit = requestData.Limit
	}

	rows, err = db.Table("reference_objects").Offset(offset).Limit(limit).
		Select(`
            reference_objects.allocation_id,
            SUM(reference_objects.size) as files_size,
            SUM(reference_objects.thumbnail_size) as thumbnails_size,
            SUM(file_stats.num_of_block_downloads) as num_of_reads,
            SUM(reference_objects.num_of_blocks) as num_of_block_writes,
            COUNT(*) as num_of_writes,
            allocations.size AS allocated_size,
            allocations.expiration_date AS expiration_date`).
		Joins(`INNER JOIN file_stats
            ON reference_objects.id = file_stats.ref_id`).
		Joins(`
            INNER JOIN allocations
            ON allocations.id = reference_objects.allocation_id`).
		Where(`reference_objects.type = 'f'
            AND reference_objects.deleted_at IS NULL`).
		Group(`reference_objects.allocation_id, allocations.expiration_date`).
		Group(`reference_objects.allocation_id, allocations.size`).
		Rows()

	if err != nil {
		Logger.Error("Error in getting the allocation stats", zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var as = &AllocationStats{}
		err = rows.Scan(&as.AllocationID, &as.FilesSize, &as.ThumbnailsSize,
			&as.NumReads, &as.BlockWrites, &as.NumWrites, &as.AllocatedSize, &as.Expiration)
		if err != nil {
			Logger.Error("Error in scanning record for blobber stats",
				zap.Error(err))
			return
		}
		as.UsedSize = as.FilesSize + as.ThumbnailsSize
		if err := as.loadAllocationDiskUsageStats(); err != nil {
			Logger.Error("AllocationStats_loadAllocationDiskUsageStats", zap.String("allocation_id", as.AllocationID), zap.Error(err))
		}
		bs.AllocationStats = append(bs.AllocationStats, as)
	}

	if err = rows.Err(); err != nil && err != sql.ErrNoRows {
		Logger.Error("Error in scanning record for blobber stats",
			zap.Error(err))
		return
	}

	var count int64
	err = db.Table("reference_objects").Where("deleted_at is null").Count(&count).Error
	if err != nil {
		Logger.Error("loadAllocationStats err where deleted_at is nul", zap.Any("err", err))
		return
	}

	if requestData != nil {
		pagination := GeneratePagination(requestData.Page, requestData.Limit, requestData.Offset, int(count))
		bs.AllocationListPagination = pagination
	}
}

func (bs *BlobberStats) loadChallengeStats(ctx context.Context) {

	var (
		db   = datastore.GetStore().GetTransaction(ctx)
		rows *sql.Rows
		err  error
	)

	rows, err = db.Table("challenges").
		Select(`COUNT(*) AS total_challenges,
		challenges.status,
		challenges.result`).
		Group(`challenges.status, challenges.result`).
		Rows()

	if err != nil {
		Logger.Error("Error in getting the blobber challenge stats",
			zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var (
			total  = int64(0)
			status = 0
			result = 0
		)

		if err = rows.Scan(&total, &status, &result); err != nil {
			Logger.Error("Error in scanning record for blobber stats",
				zap.Error(err))
			return
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

	if err = rows.Err(); err != nil && err != sql.ErrNoRows {
		Logger.Error("Error in scanning record for blobber stats",
			zap.Error(err))
		return
	}

}

func (bs *BlobberStats) loadAllocationChallengeStats(ctx context.Context) {
	var (
		db   = datastore.GetStore().GetTransaction(ctx)
		rows *sql.Rows
		err  error
	)

	rows, err = db.Table("challenges").Select(`
        challenges.allocation_id,
        COUNT(*) AS total_challenges,
        challenges.status,
        challenges.result`).
		Group(`challenges.allocation_id, challenges.status, challenges.result`).
		Rows()

	if err != nil {
		Logger.Error("Error in getting the allocation challenge stats",
			zap.Error(err))
		return
	}
	defer rows.Close()

	var allocationStatsMap = make(map[string]*AllocationStats)

	for _, as := range bs.AllocationStats {
		allocationStatsMap[as.AllocationID] = as
	}

	for rows.Next() {
		var (
			total        = int64(0)
			status       = 0
			result       = 0
			allocationID = ""
		)

		err = rows.Scan(&allocationID, &total, &status, &result)
		if err != nil {
			Logger.Error("Error in scanning record for blobber stats",
				zap.Error(err))
			return
		}

		var as = allocationStatsMap[allocationID]
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

	if err = rows.Err(); err != nil && err != sql.ErrNoRows {
		Logger.Error("Error in scanning record for blobber stats",
			zap.Error(err))
		return
	}

}

func loadAllocationList(ctx context.Context) (interface{}, error) {

	var (
		allocations = make([]AllocationId, 0)
		db          = datastore.GetStore().GetTransaction(ctx)
		rows        *sql.Rows
		err         error
	)

	rows, err = db.Table("reference_objects").
		Select("reference_objects.allocation_id").
		Group("reference_objects.allocation_id").
		Rows()

	if err != nil {
		Logger.Error("Error in getting the allocation list", zap.Error(err))
		return nil, common.NewError("get_allocations_list_failed",
			"Failed to get allocation list from DB")
	}
	defer rows.Close()

	for rows.Next() {
		var allocationId AllocationId
		if err = rows.Scan(&allocationId.Id); err != nil {
			Logger.Error("Error in scanning record for blobber allocations",
				zap.Error(err))
			return nil, common.NewError("get_allocations_list_failed",
				"Failed to scan allocation from DB")
		}
		allocations = append(allocations, allocationId)
	}

	if err = rows.Err(); err != nil && err != sql.ErrNoRows {
		Logger.Error("Error in scanning record for blobber allocations",
			zap.Error(err))
		return nil, common.NewError("get_allocations_list_failed",
			"Failed to scan allocations from DB")
	}

	return allocations, nil
}

type ReadMarkerEntity struct {
	ReadCounter          int64          `gorm:"column:counter" json:"counter"`
	LatestRedeemedRMBlob datatypes.JSON `gorm:"column:latest_redeemed_rm"`
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
	if len(rme.LatestRedeemedRMBlob) > 0 {
		err = json.Unmarshal([]byte(rme.LatestRedeemedRMBlob), prev)
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

	if err = rows.Err(); err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	return
}
