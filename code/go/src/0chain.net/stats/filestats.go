package stats

import (
	"context"

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
	NumSuccessChallenges     int64  `json:"num_of_challenges"`
	LastChallengeResponseTxn string `json:"last_challenge_txn"`
}

var fileStatsEntityMetaData *datastore.EntityMetadataImpl

/*Provider - entity provider for client object */
func FileStatsProvider() datastore.Entity {
	t := &FileStats{}
	return t
}

func SetupFileStatsEntity(store datastore.Store) {
	fileStatsEntityMetaData = datastore.MetadataProvider()
	fileStatsEntityMetaData.Name = "filestats"
	fileStatsEntityMetaData.DB = "filestats"
	fileStatsEntityMetaData.Provider = FileStatsProvider
	fileStatsEntityMetaData.Store = store

	datastore.RegisterEntityMetadata("filestats", fileStatsEntityMetaData)
}

func (fr *FileStats) GetEntityMetadata() datastore.EntityMetadata {
	return fileStatsEntityMetaData
}
func (fr *FileStats) SetKey(key datastore.Key) {
	//wm.ID = datastore.ToString(key)
}

func (fr *FileStats) GetKey() string {
	return fr.GetEntityMetadata().GetDBName() + ":" + reference.GetReferenceLookup(fr.AllocationID, fr.Path)
}

func (fr *FileStats) Read(ctx context.Context, key datastore.Key) error {
	return fr.GetEntityMetadata().GetStore().Read(ctx, key, fr)
}
func (fr *FileStats) Write(ctx context.Context) error {
	return fr.GetEntityMetadata().GetStore().Write(ctx, fr)
}
func (fr *FileStats) Delete(ctx context.Context) error {
	return nil
}

func GetFileStats(ctx context.Context, path_hash string) (*FileStats, error) {
	if len(path_hash) == 0 {
		return nil, common.NewError("invalid_paramaters", "Invalid parameters for file stats")
	}
	fs := FileStatsProvider().(*FileStats)
	err := fs.Read(ctx, fs.GetEntityMetadata().GetDBName()+":"+path_hash)
	if err != nil {
		return nil, err
	}
	return fs, err
}

func FileBlockDownloaded(ctx context.Context, allocationID string, path string) (*FileStats, error) {
	if len(allocationID) == 0 || len(path) == 0 {
		return nil, common.NewError("invalid_paramaters", "Invalid parameters for file stats")
	}
	fs := FileStatsProvider().(*FileStats)
	fs.Path = path
	fs.AllocationID = allocationID
	mutex := lock.GetMutex(fs.GetKey())
	mutex.Lock()
	defer mutex.Unlock()
	err := fs.Read(ctx, fs.GetKey())
	if err != nil {
		return nil, err
	}
	fs.NumBlockDownloads++
	err = fs.Write(ctx)
	return fs, err
}

func FileUpdated(ctx context.Context, allocationID string, path string, writeMarkerKey string) (*FileStats, error) {
	if len(allocationID) == 0 || len(path) == 0 {
		return nil, common.NewError("invalid_paramaters", "Invalid parameters for file stats")
	}
	fs := FileStatsProvider().(*FileStats)
	fs.Path = path
	fs.AllocationID = allocationID
	mutex := lock.GetMutex(fs.GetKey())
	mutex.Lock()
	defer mutex.Unlock()
	err := fs.Read(ctx, fs.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		return nil, err
	}
	fs.NumUpdates++
	fs.WriteMarker = writeMarkerKey
	err = fs.Write(ctx)
	return fs, err
}

func FileChallenged(ctx context.Context, allocationID string, path string, challengeRedeemTxn string) (*FileStats, error) {
	if len(allocationID) == 0 || len(path) == 0 {
		return nil, common.NewError("invalid_paramaters", "Invalid parameters for file stats")
	}
	fs := FileStatsProvider().(*FileStats)
	fs.Path = path
	fs.AllocationID = allocationID
	mutex := lock.GetMutex(fs.GetKey())
	mutex.Lock()
	defer mutex.Unlock()
	err := fs.Read(ctx, fs.GetKey())
	if err != nil {
		return nil, err
	}
	fs.NumSuccessChallenges++
	fs.LastChallengeResponseTxn = challengeRedeemTxn
	err = fs.Write(ctx)
	return fs, err
}
