package stats

import (
	"context"
	"encoding/json"
	"sync"

	"0chain.net/lock"

	"0chain.net/common"
	"0chain.net/datastore"
	"0chain.net/reference"
)

type FileStats struct {
	AllocationID             string `json:"allocation_id"`
	Path                     string `json:"path"`
	WriteMarker              string `json:"latest_file_write_marker"`
	WriteMarkerRedeemTxn     string
	NumUpdates               int64  `json:"num_of_updates"`
	NumBlockDownloads        int64  `json:"num_of_block_downloads"`
	SuccessChallenges        int64  `json:"num_of_challenges"`
	FailedChallenges         int64  `json:"num_of_failed_challenges"`
	LastChallengeResponseTxn string `json:"last_challenge_txn"`
}

var statsStore datastore.Store

func GetStatsStore() datastore.Store {
	return statsStore
}

func SetupStatsEntity(store datastore.Store) {
	statsStore = store
}

func (fr *FileStats) GetKey() string {
	return "filestats:" + reference.GetReferenceLookup(fr.AllocationID, fr.Path)
}

func NewSyncFileStats(allocationID string, path string) (*FileStats, *sync.Mutex) {
	fs := &FileStats{}
	fs.AllocationID = allocationID
	fs.Path = path
	mutex := lock.GetMutex(fs.GetKey())
	mutex.Lock()
	return fs, mutex
}

func GetFileStats(ctx context.Context, path_hash string) (*FileStats, error) {
	if len(path_hash) == 0 {
		return nil, common.NewError("invalid_paramaters", "Invalid parameters for file stats")
	}
	fs := &FileStats{}
	fsbytes, err := GetStatsStore().ReadBytes(ctx, "filestats:"+path_hash)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(fsbytes, fs)
	if err != nil {
		return nil, err
	}
	return fs, err
}

func (fs *FileStats) NewWrite(ctx context.Context, f *FileUploadedEvent) error {
	fsbytes, err := GetStatsStore().ReadBytes(ctx, fs.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		return err
	}
	err = json.Unmarshal(fsbytes, fs)

	fs.NumUpdates++
	fs.WriteMarker = f.WriteMarkerKey

	fsbytes, err = json.Marshal(fs)
	if err != nil {
		return err
	}
	err = GetStatsStore().WriteBytes(ctx, fs.GetKey(), fsbytes)
	if err != nil {
		return err
	}
	return nil
}

func (fs *FileStats) NewBlockDownload(ctx context.Context, f *FileDownloadedEvent) error {
	fsbytes, err := GetStatsStore().ReadBytes(ctx, fs.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		return err
	}
	if err == datastore.ErrKeyNotFound {
		return nil
	}
	err = json.Unmarshal(fsbytes, fs)
	if err != nil {
		return err
	}
	fs.NumBlockDownloads++

	fsbytes, err = json.Marshal(fs)
	if err != nil {
		return err
	}
	err = GetStatsStore().WriteBytes(ctx, fs.GetKey(), fsbytes)
	if err != nil {
		return err
	}
	return nil
}

func (fs *FileStats) ChallengeRedeemed(ctx context.Context, ch *ChallengeEvent) error {
	fsbytes, err := GetStatsStore().ReadBytes(ctx, fs.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		return err
	}
	if err == datastore.ErrKeyNotFound {
		return nil
	}
	err = json.Unmarshal(fsbytes, fs)
	if err != nil {
		return err
	}

	if ch.Result == SUCCESS {
		fs.SuccessChallenges++
	}
	if ch.Result == FAILED {
		fs.FailedChallenges++
	}
	fs.LastChallengeResponseTxn = ch.RedeemTxn

	fsbytes, err = json.Marshal(fs)
	if err != nil {
		return err
	}
	err = GetStatsStore().WriteBytes(ctx, fs.GetKey(), fsbytes)
	if err != nil {
		return err
	}
	return nil
}

// func FileChallenged(allocationID string, path string, challengeRedeemTxn string) (*FileStats, error) {
// 	if len(allocationID) == 0 || len(path) == 0 {
// 		return nil, common.NewError("invalid_paramaters", "Invalid parameters for file stats")
// 	}
// 	ctx := common.GetRootContext()
// 	fs := FileStatsProvider().(*FileStats)
// 	fs.Path = path
// 	fs.AllocationID = allocationID
// 	mutex := lock.GetMutex(fs.GetKey())
// 	mutex.Lock()
// 	defer mutex.Unlock()

// 	nctx := fileStatsEntityMetaData.GetStore().WithConnection(ctx)
// 	defer fileStatsEntityMetaData.GetStore().Discard(nctx)

// 	err := fs.Read(nctx, fs.GetKey())
// 	if err != nil {
// 		return nil, err
// 	}
// 	fs.NumSuccessChallenges++
// 	fs.LastChallengeResponseTxn = challengeRedeemTxn
// 	err = fs.Write(nctx)
// 	if err != nil {
// 		return nil, err
// 	}
// 	err = fileStatsEntityMetaData.GetStore().Commit(nctx)
// 	if err != nil {
// 		Logger.Error("Error committing the file download stats")
// 	}
// 	return fs, nil
// }
