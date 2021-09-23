package challenge

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/lock"

	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

// SetupWorkers start challenge workers
func SetupWorkers(ctx context.Context) {
	go startSyncChallenges(ctx)
	go startProcessChallenges(ctx)
	go SubmitProcessedChallenges(ctx) //nolint:errcheck // goroutines
}

// LoadValidationTickets load validation tickets for challenge
func LoadValidationTickets(ctx context.Context, challengeObj *ChallengeEntity) error {
	mutex := lock.GetMutex(challengeObj.TableName(), challengeObj.ChallengeID)
	mutex.Lock()

	defer func() {
		if r := recover(); r != nil {
			Logger.Error("[recover] LoadValidationTickets", zap.Any("err", r))
		}
	}()

	err := challengeObj.LoadValidationTickets(ctx)
	if err != nil {
		Logger.Error("Error getting the validation tickets", zap.Error(err), zap.String("challenge_id", challengeObj.ChallengeID))
	}

	return err
}

func SubmitProcessedChallenges(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			rctx := datastore.GetStore().CreateTransaction(ctx)
			db := datastore.GetStore().GetTransaction(rctx)

			rows, err := db.Table("challenges").
				Select("commit_txn_id, sequence").
				Where(ChallengeEntity{Status: Committed}).
				Order("sequence desc").Limit(1).Rows()

			if rows != nil && err == nil {
				lastSeq := 0
				lastCommitTxn := ""
				for rows.Next() {
					if err := rows.Scan(&lastCommitTxn, &lastSeq); err != nil {
						Logger.Error("Rows_Scan", zap.Error(err))
					}
				}

				openchallenges := make([]*ChallengeEntity, 0)

				db.Where(ChallengeEntity{Status: Processed}).
					Where("sequence > ?", lastSeq).
					Order("sequence").
					Find(&openchallenges)

				if len(openchallenges) > 0 {
					for _, openchallenge := range openchallenges {
						Logger.Info("Attempting to commit challenge", zap.Any("challenge_id", openchallenge.ChallengeID), zap.Any("openchallenge", openchallenge))
						if err := openchallenge.UnmarshalFields(); err != nil {
							Logger.Error("ChallengeEntity_UnmarshalFields", zap.String("challenge_id", openchallenge.ChallengeID), zap.Error(err))
						}
						mutex := lock.GetMutex(openchallenge.TableName(), openchallenge.ChallengeID)
						mutex.Lock()
						redeemCtx := datastore.GetStore().CreateTransaction(ctx)
						err := openchallenge.CommitChallenge(redeemCtx, false)
						if err != nil {
							Logger.Error("Error committing to blockchain",
								zap.Error(err),
								zap.String("challenge_id", openchallenge.ChallengeID))
						}
						mutex.Unlock()
						db := datastore.GetStore().GetTransaction(redeemCtx)
						db.Commit()
						if err == nil && openchallenge.Status == Committed {
							Logger.Info("Challenge has been submitted to blockchain",
								zap.Any("id", openchallenge.ChallengeID),
								zap.String("txn", openchallenge.CommitTxnID))
						} else {
							Logger.Info("Challenge was not committed", zap.Any("challenge_id", openchallenge.ChallengeID))
							break
						}
					}
				}
				db.Rollback()
				rctx.Done()

				rctx = datastore.GetStore().CreateTransaction(ctx)
				db = datastore.GetStore().GetTransaction(rctx)
				toBeVerifiedChallenges := make([]*ChallengeEntity, 0)
				// commit challenges on local state for all challenges that
				// have missed the commit txn from blockchain
				db.Where(ChallengeEntity{Status: Processed}).
					Where("sequence < ?", lastSeq).
					Find(&toBeVerifiedChallenges)

				for _, toBeVerifiedChallenge := range toBeVerifiedChallenges {
					Logger.Info("Attempting to commit challenge through verification", zap.Any("challenge_id", toBeVerifiedChallenge.ChallengeID), zap.Any("openchallenge", toBeVerifiedChallenge))
					if err := toBeVerifiedChallenge.UnmarshalFields(); err != nil {
						Logger.Error("ChallengeEntity_UnmarshalFields", zap.String("challenge_id", toBeVerifiedChallenge.ChallengeID), zap.Error(err))
					}
					mutex := lock.GetMutex(toBeVerifiedChallenge.TableName(), toBeVerifiedChallenge.ChallengeID)
					mutex.Lock()
					redeemCtx := datastore.GetStore().CreateTransaction(ctx)
					err := toBeVerifiedChallenge.CommitChallenge(redeemCtx, true)
					if err != nil {
						Logger.Error("Error committing to blockchain",
							zap.Error(err),
							zap.String("challenge_id", toBeVerifiedChallenge.ChallengeID))
					}
					mutex.Unlock()
					db := datastore.GetStore().GetTransaction(redeemCtx)
					db.Commit()
					if err == nil && toBeVerifiedChallenge.Status == Committed {
						Logger.Info("Challenge has been submitted to blockchain",
							zap.Any("id", toBeVerifiedChallenge.ChallengeID),
							zap.String("txn", toBeVerifiedChallenge.CommitTxnID))
					} else {
						Logger.Info("Challenge was not committed after verification", zap.Any("challenge_id", toBeVerifiedChallenge.ChallengeID))
					}
				}

				db.Rollback()
				rctx.Done()
			} else {
				Logger.Error("Error in getting the challenges for blockchain processing.",
					zap.Error(err))
			}
		}
		time.Sleep(time.Duration(config.Configuration.ChallengeResolveFreq) * time.Second)
	}
}

func startProcessChallenges(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.Configuration.ChallengeResolveFreq) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processChallenges(ctx)
		}
	}
}

// startSyncChallenges
func startSyncChallenges(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.Configuration.ChallengeResolveFreq) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			syncChallenges(ctx)
		}
	}
}
