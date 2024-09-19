package datastore

import (
	"context"
	"database/sql"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/common/core/util/storage"
	"github.com/0chain/common/core/util/storage/kv"
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/bloom"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type contextKey int

const (
	ContextKeyTransaction contextKey = iota
	ContextKeyStore
)

type CommitToCahe func(tx *EnhancedDB)

type EnhancedDB struct {
	SessionCache     map[string]interface{}
	CommitAllocCache CommitToCahe
	*gorm.DB
}

func EnhanceDB(db *gorm.DB) *EnhancedDB {
	cache := make(map[string]interface{})
	return &EnhancedDB{DB: db, SessionCache: cache}
}

func (edb *EnhancedDB) Commit() *gorm.DB {
	now := time.Now()
	db := edb.DB.Commit()
	if db.Error == nil {
		if edb.CommitAllocCache != nil {
			elapsedCommit := time.Since(now)
			edb.CommitAllocCache(edb)
			logging.Logger.Info("dbCommit", zap.Duration("elapsedDB", elapsedCommit), zap.Duration(
				"elapsedCommitAlloc", time.Since(now)-elapsedCommit,
			), zap.Duration("total", time.Since(now)))
		}
	}
	return db
}

type Store interface {

	// GetDB get raw gorm db
	GetDB() *gorm.DB
	// CreateTransaction create transaction, and save it in context
	CreateTransaction(ctx context.Context, opts ...*sql.TxOptions) context.Context
	// GetTransaction get transaction from context
	GetTransaction(ctx context.Context) *EnhancedDB
	WithNewTransaction(f func(ctx context.Context) error, opts ...*sql.TxOptions) error
	WithTransaction(ctx context.Context, f func(ctx context.Context) error) error
	// Get db connection with user that creates roles and databases. Its dialactor does not contain database name
	GetPgDB() (*gorm.DB, error)
	Open() error
	Close()
}

var (
	instance      Store
	blockInstance storage.StorageAdapter
)

func init() {
	instance = &postgresStore{}
}

func GetStore() Store {
	return instance
}

func GetBlockStore() storage.StorageAdapter {
	return blockInstance
}

func OpenBlockStore() error {
	//TODO: read from config
	pebbleDir := config.Configuration.PebbleDir
	opts := &pebble.Options{
		Cache:                    pebble.NewCache(config.Configuration.PebbleCache),
		WALDir:                   config.Configuration.PebbleWALDir,
		MemTableSize:             uint64(config.Configuration.PebbleMemtableSize),
		MaxOpenFiles:             config.Configuration.PebbleMaxOpenFiles,
		BytesPerSync:             1024 * 1024, //1MB
		MaxConcurrentCompactions: func() int { return 4 },
		Levels: []pebble.LevelOptions{
			{TargetFileSize: 4 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
			{TargetFileSize: 8 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
			{TargetFileSize: 16 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
			{TargetFileSize: 32 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
			{TargetFileSize: 64 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
			{TargetFileSize: 128 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
			{TargetFileSize: 256 * 1024 * 1024},
		},
		WALBytesPerSync:             512 * 1024,       // 512kb
		LBaseMaxBytes:               64 * 1024 * 1024, // 64MB
		MemTableStopWritesThreshold: 4,
	}
	pebbleInstance, err := kv.NewPebbleAdapter(pebbleDir, opts)
	if err != nil {
		return err
	}
	blockInstance = pebbleInstance
	return nil
}

func FromContext(ctx context.Context) Store {
	store := ctx.Value(ContextKeyStore)
	if store != nil {
		return store.(Store)
	}

	return GetStore()
}
