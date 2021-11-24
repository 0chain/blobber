package handler

import (
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

func SetDBStats() error {
	db := datastore.GetStore().GetDB()
	sqldb, err := db.DB()
	if err != nil {
		return common.NewErrorf("db_open_error", "Error opening the DB connection: %v", err)
	}

	sqlDBStats := sqldb.Stats()

	err = stats.ConvertSqlDBStatsToCustomDBStats(&sqlDBStats)
	if err != nil {
		stats.SetDBStatStatusFail()
		return err
	}

	stats.SetDBStatStatusOK()

	return nil
}
