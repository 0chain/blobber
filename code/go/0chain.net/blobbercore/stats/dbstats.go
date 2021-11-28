package stats

import (
	"database/sql"
	"encoding/json"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
	"time"
)

// customDBStats contains database statistics.
type customDBStats struct {
	MaxOpenConnections int // Maximum number of open connections to the database.

	// Pool Status
	OpenConnections int // The number of established connections both in use and idle.
	InUse           int // The number of connections currently in use.
	Idle            int // The number of idle connections.

	// Counters
	WaitCount         int64         // The total number of connections waited for.
	WaitDuration      time.Duration // The total time blocked waiting for a new connection.
	MaxIdleClosed     int64         // The total number of connections closed due to SetMaxIdleConns.
	MaxIdleTimeClosed int64         // The total number of connections closed due to SetConnMaxIdleTime.
	MaxLifetimeClosed int64         // The total number of connections closed due to SetConnMaxLifetime.
	Status            string
}

func convertSqlDBStatsToCustomDBStats(in *sql.DBStats, out *customDBStats) error {
	b, err := json.Marshal(in)
	if err != nil {
		logging.Logger.Error("Failed to marshal sql.DBStats", zap.Any("err", err))
		return err
	}

	err = json.Unmarshal(b, out)
	if err != nil {
		return common.NewErrorf("struct_copy_error", "Error copying struct: %v", err)
	}

	return nil
}
