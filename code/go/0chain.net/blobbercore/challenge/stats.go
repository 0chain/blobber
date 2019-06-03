package challenge

import (
	"context"

	"0chain.net/blobbercore/datastore"
	"0chain.net/blobbercore/stats"

	"github.com/jinzhu/gorm"
)

func FileChallenged(ctx context.Context, refID int64, result ChallengeResult, challengeTxn string) {
	db := datastore.GetStore().GetTransaction(ctx)
	stats := &stats.FileStats{RefID: refID}
	if result == ChallengeSuccess {
		db.Table(stats.TableName()).Where(stats).Updates(map[string]interface{}{"num_of_challenges": gorm.Expr("num_of_challenges + ?", 1), "last_challenge_txn": challengeTxn})
	} else if result == ChallengeFailure {
		db.Table(stats.TableName()).Where(stats).Updates(map[string]interface{}{"num_of_failed_challenges": gorm.Expr("num_of_failed_challenges + ?", 1), "last_challenge_txn": challengeTxn})
	}
}
