package stats

import (
	"context"
	"encoding/json"
	"math"
	"sync"

	"0chain.net/datastore"
	"0chain.net/filestore"
	"0chain.net/lock"
)

type AllocationStats struct {
	AllocationID      string            `json:"allocation_id"`
	GivenUpChallenges map[string]string `json:"given_up_challenges"`
	TempFolderSize    int64             `json:"-"`
	Stats
}

func LoadAllocationStatsFromBytes(ctx context.Context, value []byte) (*AllocationStats, error) {
	fs := &AllocationStats{}
	err := json.Unmarshal(value, fs)
	if err != nil {
		return nil, err
	}
	du, err := filestore.GetFileStore().GetlDiskSizeUsed(fs.AllocationID)
	if err != nil {
		du = -1
	}
	fs.DiskSizeUsed = du
	tfs, err := filestore.GetFileStore().GetTempPathSize(fs.AllocationID)
	if err != nil {
		tfs = -1
	}
	fs.TempFolderSize = tfs
	return fs, nil
}

func NewSyncAllocationStats(allocationID string) (*AllocationStats, *sync.Mutex) {
	fs := &AllocationStats{}
	fs.AllocationID = allocationID
	fs.GivenUpChallenges = make(map[string]string)
	mutex := lock.GetMutex(fs.GetKey())
	mutex.Lock()
	return fs, mutex
}

func (as *AllocationStats) GetKey() datastore.Key {
	return "allocationstats:" + as.AllocationID
}

func (as *AllocationStats) NewChallenge(ctx context.Context, ch *ChallengeEvent) error {
	fsbytes, err := GetStatsStore().ReadBytes(ctx, as.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		return err
	}
	err = json.Unmarshal(fsbytes, as)

	as.OpenChallenges++
	as.TotalChallenges++

	fsbytes, err = json.Marshal(as)
	if err != nil {
		return err
	}
	err = GetStatsStore().WriteBytes(ctx, as.GetKey(), fsbytes)
	if err != nil {
		return err
	}
	return nil
}

func (as *AllocationStats) ChallengeRedeemed(ctx context.Context, ch *ChallengeEvent) error {
	fsbytes, err := GetStatsStore().ReadBytes(ctx, as.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		return err
	}
	err = json.Unmarshal(fsbytes, as)

	as.OpenChallenges--
	if ch.Result == SUCCESS {
		as.SuccessChallenges++
	}
	if ch.Result == FAILED {
		as.FailedChallenges++
	}

	if ch.RedeemStatus == REDEEMSUCCESS {
		as.RedeemSuccessChallenges++
	}
	if ch.RedeemStatus == REDEEMERROR {
		as.RedeemErrorChallenges++
		as.GivenUpChallenges[ch.ChallengeID] = ch.ChallengeID
	}

	fsbytes, err = json.Marshal(as)
	if err != nil {
		return err
	}
	err = GetStatsStore().WriteBytes(ctx, as.GetKey(), fsbytes)
	if err != nil {
		return err
	}
	return nil
}

func (as *AllocationStats) NewWrite(ctx context.Context, f *FileUploadedEvent) error {
	fsbytes, err := GetStatsStore().ReadBytes(ctx, as.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		return err
	}
	err = json.Unmarshal(fsbytes, as)

	as.NumWrites++
	as.UsedSize += f.Size
	if f.Operation == DELETE_OPERATION {
		as.BlockWrites += int64(math.Floor(float64(f.Size*1.0) / filestore.CHUNK_SIZE))
	} else {
		as.BlockWrites += int64(math.Ceil(float64(f.Size*1.0) / filestore.CHUNK_SIZE))
	}

	fsbytes, err = json.Marshal(as)
	if err != nil {
		return err
	}
	err = GetStatsStore().WriteBytes(ctx, as.GetKey(), fsbytes)
	if err != nil {
		return err
	}
	return nil
}

func (as *AllocationStats) NewBlockDownload(ctx context.Context, f *FileDownloadedEvent) error {
	fsbytes, err := GetStatsStore().ReadBytes(ctx, as.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		return err
	}
	err = json.Unmarshal(fsbytes, as)
	if err != nil {
		return err
	}
	as.NumReads++

	fsbytes, err = json.Marshal(as)
	if err != nil {
		return err
	}
	err = GetStatsStore().WriteBytes(ctx, as.GetKey(), fsbytes)
	if err != nil {
		return err
	}
	return nil
}
