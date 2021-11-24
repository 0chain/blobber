package stats

import (
	"database/sql"
	"encoding/json"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"log"
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

var myCustomDBStats *customDBStats

func init() {
	myCustomDBStats = &customDBStats{Status: "connecting"}
}

func SetDBStatStatusFail() {
	myCustomDBStats = &customDBStats{Status: "✗"}
}

func SetDBStatStatusOK() {
	myCustomDBStats.Status = "✔"
}

func ConvertSqlDBStatsToCustomDBStats(in *sql.DBStats) error {
	err := toSubStruct(&in, myCustomDBStats)
	if err != nil {
		return common.NewErrorf("struct_copy_error", "Error copying struct: %v", err)
	}

	return nil
}

func toSubStruct(in, out interface{}) error {
	b, err := json.Marshal(in)
	if err != nil {
		log.Println(err)
		return err
	}

	err = json.Unmarshal(b, out)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func GetDBStats() (*customDBStats, error) {
	db := datastore.GetStore().GetDB()
	sqldb, err := db.DB()
	if err != nil {
		SetDBStatStatusFail()
		return nil, common.NewErrorf("db_open_error", "Error opening the DB connection: %v", err)
	}

	sqlDBStats := sqldb.Stats()

	err = ConvertSqlDBStatsToCustomDBStats(&sqlDBStats)
	if err != nil {
		SetDBStatStatusFail()
		return nil, err
	}

	SetDBStatStatusOK()

	return myCustomDBStats, nil
}
