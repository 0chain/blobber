package datastore

import (
	"context"
	"runtime/debug"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type contextKey int

const (
	ContextKeyTransaction contextKey = iota
	ContextKeyStore
)

var (
	txs   map[string]holder
	guard sync.Mutex
)

type holder struct {
	stack   string
	created int64
}

func init() {
	txs = make(map[string]holder)

	go func() {
		tick := time.Tick(30 * time.Second)

		for {
			select {
			case <-tick:
				guard.Lock()
				t := txs
				guard.Unlock()
				for _, v := range t {
					if time.Now().UnixNano()-v.created > 30*time.Second.Nanoseconds() {
						logging.Logger.Error("stale_transaction", zap.String("stack", v.stack))
					}
				}
			}
		}
	}()
}

type EnhancedDB struct {
	id           string
	SessionCache map[string]interface{}
	*gorm.DB
}

func (e *EnhancedDB) Commit() *gorm.DB {
	guard.Lock()
	delete(txs, e.id)
	guard.Unlock()

	return e.DB.Commit()
}
func (e *EnhancedDB) Rollback() *gorm.DB {
	guard.Lock()
	delete(txs, e.id)
	guard.Unlock()

	return e.DB.Rollback()
}

func EnhanceDB(db *gorm.DB) *EnhancedDB {
	newUUID, _ := uuid.NewUUID()
	guard.Lock()
	txs[newUUID.String()] = holder{string(debug.Stack()), time.Now().UnixNano()}
	guard.Unlock()

	cache := make(map[string]interface{})
	return &EnhancedDB{DB: db, SessionCache: cache}
}

type Store interface {

	// GetDB get raw gorm db
	GetDB() *gorm.DB
	// CreateTransaction create transaction, and save it in context
	CreateTransaction(ctx context.Context) context.Context
	// GetTransaction get transaction from context
	GetTransaction(ctx context.Context) *EnhancedDB
	WithNewTransaction(f func(ctx context.Context) error) error
	WithTransaction(ctx context.Context, f func(ctx context.Context) error) error
	// Get db connection with user that creates roles and databases. Its dialactor does not contain database name
	GetPgDB() (*gorm.DB, error)
	Open() error
	Close()
}

var instance Store

func init() {
	instance = &postgresStore{}
}

func GetStore() Store {
	return instance
}

func FromContext(ctx context.Context) Store {
	store := ctx.Value(ContextKeyStore)
	if store != nil {
		return store.(Store)
	}

	return GetStore()
}
