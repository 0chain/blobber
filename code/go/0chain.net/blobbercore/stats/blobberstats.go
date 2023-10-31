package stats

import (
	"context"
	"database/sql"
	"errors"
	"runtime"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"go.uber.org/zap"
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
	AllocatedSize      int64  `json:"allocated_size"`
	UsedSize           int64  `json:"used_size"`
	FilesSize          int64  `json:"files_size"`
	ThumbnailsSize     int64  `json:"thumbnails_size"`
	DiskSizeUsed       uint64 `json:"disk_size_used"`
	BlockWrites        int64  `json:"num_of_block_writes"`
	NumWrites          int64  `json:"num_of_writes"`
	NumReads           int64  `json:"num_of_reads"`
	TotalChallenges    int64  `json:"total_challenges"`
	OpenChallenges     int64  `json:"num_open_challenges"`
	SuccessChallenges  int64  `json:"num_success_challenges"`
	FailedChallenges   int64  `json:"num_failed_challenges"`
	RedeemedChallenges int64  `json:"num_redeemed_challenges"`
}

type Duration int64

func (d Duration) String() string {
	return (time.Duration(d) * time.Second).String()
}

type BlobberStats struct {
	Stats
	NumAllocation             int64             `json:"num_of_allocations"`
	ClientID                  string            `json:"-"`
	PublicKey                 string            `json:"-"`
	InfraStats                InfraStats        `json:"-"`
	DBStats                   *DBStats          `json:"-"`
	FailedChallengeList       []ChallengeEntity `json:"-"`
	FailedChallengePagination *Pagination       `json:"failed_challenge_pagination,omitempty"`
	AllocationListPagination  *Pagination       `json:"allocation_list_pagination,omitempty"`

	// configurations
	Capacity   int64   `json:"capacity"`
	ReadPrice  float64 `json:"read_price"`
	WritePrice float64 `json:"write_price"`

	AllocationStats []*AllocationStats `json:"-"`

	// total for all allocations
	ReadMarkers  ReadMarkersStat  `json:"read_markers"`
	WriteMarkers WriteMarkersStat `json:"write_markers"`
}

var fs *BlobberStats

const statsHandlerPeriod = 30 * time.Minute

type AllocationId struct {
	Id string `json:"id"`
}

func SetupStatsWorker(ctx context.Context) {
	fs = &BlobberStats{}
	go func() {
		_ = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
			fs.loadBasicStats(ctx)
			fs.loadDetailedStats(ctx)
			fs.loadFailedChallengeList(ctx)
			return common.NewError("rollback", "read_only")
		})
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(statsHandlerPeriod):
				_ = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
					fs.loadBasicStats(ctx)
					fs.loadDetailedStats(ctx)
					fs.loadFailedChallengeList(ctx)
					return common.NewError("rollback", "read_only")
				})
			}
		}
	}()
}

func LoadBlobberStats(ctx context.Context) *BlobberStats {
	fs.loadInfraStats(ctx)
	fs.loadDBStats()
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
	//
	du := filestore.GetFileStore().GetTotalFilesSize()

	bs.DiskSizeUsed = du
	bs.loadStats(ctx)
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

	//todo hide inside wrapper to db
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

	fcs, count, err := getAllFailedChallenges(ctx, fcrd.Offset, fcrd.Limit)
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
	COALESCE (SUM (reference_objects.num_of_block_downloads), 0) AS num_of_reads,
	COALESCE (SUM (reference_objects.num_of_blocks), 0) AS num_of_block_writes,
	COUNT (*) AS num_of_writes`

	var (
		db  = datastore.GetStore().GetTransaction(ctx)
		row *sql.Row
		err error
	)

	row = db.Table("reference_objects").
		Select(sel).
		Where("reference_objects.type = 'f'").
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
            SUM(reference_objects.num_of_block_downloads) as num_of_reads,
            SUM(reference_objects.num_of_blocks) as num_of_block_writes,
            COUNT(*) as num_of_writes,
            allocations.blobber_size AS allocated_size,
            allocations.expiration_date AS expiration_date`).
		Joins(`
            INNER JOIN allocations
            ON allocations.id = reference_objects.allocation_id`).
		Where(`reference_objects.type = 'f' AND allocations.finalized = 'f'`).
		Group(`reference_objects.allocation_id, allocations.expiration_date`).
		Group(`reference_objects.allocation_id, allocations.blobber_size`).
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

	if err = rows.Err(); err != nil && !errors.Is(err, sql.ErrNoRows) {
		Logger.Error("Error in scanning record for blobber stats",
			zap.Error(err))
		return
	}

	if requestData != nil {
		pagination := GeneratePagination(requestData.Page, requestData.Limit, requestData.Offset, len(bs.AllocationStats))
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

		if result == 1 && status == 3 {
			bs.SuccessChallenges += total
			bs.RedeemedChallenges += total
		} else if result == 2 || status == 4 {
			bs.FailedChallenges += total
		} else if status != 3 {
			bs.OpenChallenges += total
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

type ReadMarkerEntity struct {
	ReadCounter      int64 `gorm:"column:counter" json:"counter"`
	LatestRedeemedRC int64 `gorm:"column:latest_redeemed_rc"`
}

func loadAllocReadMarkersStat(ctx context.Context, allocationID string) (*ReadMarkersStat, error) {
	var (
		db  = datastore.GetStore().GetTransaction(ctx)
		rme ReadMarkerEntity
	)

	row := db.Table("read_markers").
		Select("counter, latest_redeemed_rc").
		Where("allocation_id = ?", allocationID).
		Limit(1).Row()

	err := row.Err()
	if err != nil {
		return nil, err
	}

	err = row.Scan(&rme.ReadCounter, &rme.LatestRedeemedRC)

	rms := &ReadMarkersStat{}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return rms, nil // empty
		}
		return nil, err
	}

	if rme.ReadCounter > rme.LatestRedeemedRC {
		rms.Pending = rme.ReadCounter - rme.LatestRedeemedRC // pending
	}

	rms.Redeemed = rme.LatestRedeemedRC // already redeemed
	return rms, nil
}

// copy pasted from writemarker package because of import cycle
type WriteMarkerStatus int

const (
	Accepted  WriteMarkerStatus = iota // 0
	Committed                          // 1
	Failed                             // 2
)

func loadAllocWriteMarkerStat(ctx context.Context, allocationID string) (wms *WriteMarkersStat, err error) {
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
