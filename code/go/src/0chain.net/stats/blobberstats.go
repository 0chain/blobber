package stats

import (
	"context"
	"encoding/json"
	"math"
	"sync"

	"0chain.net/config"
	"0chain.net/datastore"
	"0chain.net/filestore"
	"0chain.net/lock"
	"0chain.net/node"
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
	RedeemErrorChallenges   int64 `json:"num_error_challenges"`
	RedeemSuccessChallenges int64 `json:"num_redeem_challenges"`
}

type BlobberStats struct {
	Stats
	NumAllocation   int64              `json:"num_of_allocations"`
	ClientID        string             `json:"-"`
	PublicKey       string             `json:"-"`
	Capacity        int64              `json:"-"`
	AllocationStats []*AllocationStats `json:"-"`
}

func (bs *BlobberStats) GetKey() datastore.Key {
	return "blobberstats:" + bs.ClientID
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
	fsbytes, err := GetStatsStore().ReadBytes(ctx, fs.GetKey())
	if err != nil {
		return fs
	}
	json.Unmarshal(fsbytes, fs)

	statshandler := func(ctx context.Context, key datastore.Key, value []byte) error {
		as, err := LoadAllocationStatsFromBytes(ctx, value)
		if err != nil {
			return nil
		}
		fs.AllocationStats = append(fs.AllocationStats, as)
		return nil
	}
	GetStatsStore().IteratePrefix(ctx, "allocationstats", statshandler)
	return fs
}

func NewSyncBlobberStats() (*BlobberStats, *sync.Mutex) {
	fs := &BlobberStats{}
	fs.ClientID = node.Self.ID
	fs.PublicKey = node.Self.PublicKey
	fs.Capacity = config.Configuration.Capacity
	mutex := lock.GetMutex(fs.GetKey())
	mutex.Lock()
	return fs, mutex
}

func (bs *BlobberStats) NewBlockDownload(ctx context.Context, f *FileDownloadedEvent) error {
	fsbytes, err := GetStatsStore().ReadBytes(ctx, bs.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		return err
	}
	err = json.Unmarshal(fsbytes, bs)

	bs.NumReads++

	fsbytes, err = json.Marshal(bs)
	if err != nil {
		return err
	}
	err = GetStatsStore().WriteBytes(ctx, bs.GetKey(), fsbytes)
	if err != nil {
		return err
	}
	return nil
}

func (bs *BlobberStats) NewWrite(ctx context.Context, f *FileUploadedEvent) error {
	fsbytes, err := GetStatsStore().ReadBytes(ctx, bs.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		return err
	}
	err = json.Unmarshal(fsbytes, bs)

	bs.NumWrites++
	bs.UsedSize += f.Size
	if f.Operation == DELETE_OPERATION {
		bs.BlockWrites += int64(math.Floor(float64(f.Size*1.0) / filestore.CHUNK_SIZE))
	} else {
		bs.BlockWrites += int64(math.Ceil(float64(f.Size*1.0) / filestore.CHUNK_SIZE))
	}

	fsbytes, err = json.Marshal(bs)
	if err != nil {
		return err
	}
	err = GetStatsStore().WriteBytes(ctx, bs.GetKey(), fsbytes)
	if err != nil {
		return err
	}
	return nil
}

func (bs *BlobberStats) NewAllocation(ctx context.Context, allocationID string) error {
	fsbytes, err := GetStatsStore().ReadBytes(ctx, bs.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		return err
	}
	err = json.Unmarshal(fsbytes, bs)

	bs.NumAllocation++

	fsbytes, err = json.Marshal(bs)
	if err != nil {
		return err
	}
	err = GetStatsStore().WriteBytes(ctx, bs.GetKey(), fsbytes)
	if err != nil {
		return err
	}
	return nil
}

func (bs *BlobberStats) NewChallenge(ctx context.Context, ch *ChallengeEvent) error {
	fsbytes, err := GetStatsStore().ReadBytes(ctx, bs.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		return err
	}
	err = json.Unmarshal(fsbytes, bs)

	bs.OpenChallenges++
	bs.TotalChallenges++

	fsbytes, err = json.Marshal(bs)
	if err != nil {
		return err
	}
	err = GetStatsStore().WriteBytes(ctx, bs.GetKey(), fsbytes)
	if err != nil {
		return err
	}
	return nil
}

func (bs *BlobberStats) ChallengeRedeemed(ctx context.Context, ch *ChallengeEvent) error {
	fsbytes, err := GetStatsStore().ReadBytes(ctx, bs.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		return err
	}
	err = json.Unmarshal(fsbytes, bs)

	bs.OpenChallenges--
	if ch.Result == SUCCESS {
		bs.SuccessChallenges++
	}
	if ch.Result == FAILED {
		bs.FailedChallenges++
	}
	if ch.RedeemStatus == REDEEMSUCCESS {
		bs.RedeemSuccessChallenges++
	}
	if ch.RedeemStatus == REDEEMERROR {
		bs.RedeemErrorChallenges++
	}

	fsbytes, err = json.Marshal(bs)
	if err != nil {
		return err
	}
	err = GetStatsStore().WriteBytes(ctx, bs.GetKey(), fsbytes)
	if err != nil {
		return err
	}
	return nil
}
