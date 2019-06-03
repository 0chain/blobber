package stats

import (
	"context"

	"0chain.net/blobbercore/config"
	"0chain.net/blobbercore/datastore"
	"0chain.net/blobbercore/filestore"
	."0chain.net/core/logging"
	"0chain.net/core/node"

	"go.uber.org/zap"
)

type Stats struct {
	UsedSize                int64 `json:"used_size"`
	DiskSizeUsed            int64 `json:"-"`
	BlockWrites             int64 `json:"num_of_block_writes"`
	NumWrites               int64 `json:"num_of_writes"`
	NumReads                int64 `json:"num_of_reads"`
	TotalChallenges         int64 `json:"total_challenges"`
	OpenChallenges          int64 `json:"num_open_challenges"`
	SuccessChallenges       int64 `json:"num_success_challenges"`
	FailedChallenges        int64 `json:"num_failed_challenges"`
	RedeemedChallenges   	int64 `json:"num_redeemed_challenges"`
}

type BlobberStats struct {
	Stats
	NumAllocation   int64              `json:"num_of_allocations"`
	ClientID        string             `json:"-"`
	PublicKey       string             `json:"-"`
	Capacity        int64              `json:"-"`
	AllocationStats []*AllocationStats `json:"-"`
}

func LoadBlobberStats(ctx context.Context) *BlobberStats {
	fs := &BlobberStats{}
	fs.AllocationStats = make([]*AllocationStats, 0)
	fs.ClientID = node.Self.ID
	fs.PublicKey = node.Self.PublicKey
	fs.Capacity = config.Configuration.Capacity
	du, err := filestore.GetFileStore().GetTotalDiskSizeUsed()
	if err != nil {
		du = -1
	}
	fs.DiskSizeUsed = du
	fs.loadStats(ctx)
	fs.loadAllocationStats(ctx)
	return fs
}

func (bs *BlobberStats) loadStats(ctx context.Context) {
	db := datastore.GetStore().GetTransaction(ctx)
	rows, err := db.Debug().Table("reference_objects").Select(
		"SUM(reference_objects.size) as used_size, SUM(file_stats.num_of_block_downloads) as num_of_reads, SUM(reference_objects.num_of_blocks) as num_of_block_writes, COUNT(*) as num_of_writes",
	).Joins("inner join file_stats on reference_objects.id = file_stats.ref_id where reference_objects.type = 'f'").Rows()

	if err != nil {
		Logger.Error("Error in getting the blobber stats", zap.Error(err))
	}
	for rows.Next() {
		err = rows.Scan(&bs.UsedSize, &bs.NumReads, &bs.BlockWrites, &bs.NumWrites)
		if err != nil {
			Logger.Error("Error in scanning record for blobber stats", zap.Error(err))
		}
		break
	}
	rows.Close()

	db.Debug().Table("allocations").Count(&bs.NumAllocation)
}

func (bs *BlobberStats) loadAllocationStats(ctx context.Context) {
	bs.AllocationStats = make([]*AllocationStats, 0)
	db := datastore.GetStore().GetTransaction(ctx)
	rows, err := db.Debug().Table("reference_objects").Select(
		"reference_objects.allocation_id, SUM(reference_objects.size) as used_size, SUM(file_stats.num_of_block_downloads) as num_of_reads, SUM(reference_objects.num_of_blocks) as num_of_block_writes, COUNT(*) as num_of_writes",
	).Joins("inner join file_stats on reference_objects.id = file_stats.ref_id where reference_objects.type = 'f'").Group("reference_objects.allocation_id").Rows()

	if err != nil {
		Logger.Error("Error in getting the allocation stats", zap.Error(err))
	}

	for rows.Next() {
		as := &AllocationStats{}
		err = rows.Scan(&as.AllocationID, &as.UsedSize, &as.NumReads, &as.BlockWrites, &as.NumWrites)
		if err != nil {
			Logger.Error("Error in scanning record for blobber stats", zap.Error(err))
		}
		as.loadAllocationDiskUsageStats()
		bs.AllocationStats = append(bs.AllocationStats, as)
	}
	rows.Close()

}
