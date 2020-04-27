package stats

import (
	"context"
	"time"

	"0chain.net/blobbercore/config"
	"0chain.net/blobbercore/datastore"
	"0chain.net/blobbercore/filestore"
	"0chain.net/core/common"
	. "0chain.net/core/logging"
	"0chain.net/core/node"

	"go.uber.org/zap"
)

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

type Duration int64

func (d Duration) String() string {
	return (time.Duration(d) * time.Second).String()
}

type BlobberStats struct {
	Stats
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
}

func (bs *BlobberStats) loadDetailedStats(ctx context.Context) {
	bs.loadAllocationStats(ctx)
	bs.loadChallengeStats(ctx)
	bs.loadAllocationChallengeStats(ctx)
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
	rows, err := db.Table("challenges").Select("COUNT(*) as total_challenges, challenges.status, challenges.result").Group("challenges.status, challenges.result").Rows()
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
	rows, err := db.Table("challenges").Select("challenges.allocation_id, COUNT(*) as total_challenges, challenges.status, challenges.result").Group("challenges.allocation_id, challenges.status, challenges.result").Rows()
	if err != nil {
		Logger.Error("Error in getting the allocation challenge stats", zap.Error(err))
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
