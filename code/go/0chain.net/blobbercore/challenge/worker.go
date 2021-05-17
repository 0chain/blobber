package challenge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	"0chain.net/blobbercore/config"
	"0chain.net/blobbercore/datastore"
	"0chain.net/core/chain"
	"0chain.net/core/lock"
	"0chain.net/core/node"
	"0chain.net/core/transaction"
	"0chain.net/core/common"

	"github.com/remeh/sizedwaitgroup"
	"gorm.io/gorm"

	. "0chain.net/core/logging"
	"go.uber.org/zap"
)

type BCChallengeResponse struct {
	BlobberID  string             `json:"blobber_id"`
	Challenges []*ChallengeEntity `json:"challenges"`
}

func SetupWorkers(ctx context.Context) {
	go FindChallenges(ctx)
	go SubmitProcessedChallenges(ctx) //nolint:errcheck // goroutines
}

func GetValidationTickets(ctx context.Context, challengeObj *ChallengeEntity) error {
	mutex := lock.GetMutex(challengeObj.TableName(), challengeObj.ChallengeID)
	mutex.Lock()
	err := challengeObj.GetValidationTickets(ctx)
	if err != nil {
		Logger.Error("Error getting the validation tickets", zap.Error(err), zap.String("challenge_id", challengeObj.ChallengeID))
	}
	mutex.Unlock()
	return err
}

func SubmitProcessedChallenges(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			Logger.Info("Attempting to commit processed challenges...")
			rctx := datastore.GetStore().CreateTransaction(ctx)
			db := datastore.GetStore().GetTransaction(rctx)
			//lastChallengeRedeemed := &ChallengeEntity{}
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

	return nil //nolint:govet // need more time to verify
}

var iterInprogress = false

func FindChallenges(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.Configuration.ChallengeResolveFreq) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !iterInprogress {
				iterInprogress = true
				rctx := datastore.GetStore().CreateTransaction(ctx)
				db := datastore.GetStore().GetTransaction(rctx)
				openchallenges := make([]*ChallengeEntity, 0)
				db.Where(ChallengeEntity{Status: Accepted}).Find(&openchallenges)
				if len(openchallenges) > 0 {
					swg := sizedwaitgroup.New(config.Configuration.ChallengeResolveNumWorkers)
					for _, openchallenge := range openchallenges {
						Logger.Info("Processing the challenge", zap.Any("challenge_id", openchallenge.ChallengeID), zap.Any("openchallenge", openchallenge))
						err := openchallenge.UnmarshalFields()
						if err != nil {
							Logger.Error("Error unmarshaling challenge entity.", zap.Error(err))
							continue
						}
						swg.Add()
						go func(redeemCtx context.Context, challengeEntity *ChallengeEntity) {
							redeemCtx = datastore.GetStore().CreateTransaction(redeemCtx)
							defer redeemCtx.Done()
							err := GetValidationTickets(redeemCtx, challengeEntity)
							if err != nil {
								Logger.Error("Getting validation tickets failed", zap.Any("challenge_id", challengeEntity.ChallengeID), zap.Error(err))
							}
							db := datastore.GetStore().GetTransaction(redeemCtx)
							err = db.Commit().Error
							if err != nil {
								Logger.Error("Error commiting the readmarker redeem", zap.Error(err))
							}
							swg.Done()
						}(ctx, openchallenge)
					}
					swg.Wait()
				}
				db.Rollback()
				rctx.Done()

				params := make(map[string]string)
				params["blobber"] = node.Self.ID

				var blobberChallenges BCChallengeResponse
				blobberChallenges.Challenges = make([]*ChallengeEntity, 0)
				retBytes, err := transaction.MakeSCRestAPICall(transaction.STORAGE_CONTRACT_ADDRESS, "/openchallenges", params, chain.GetServerChain(), nil)

				if err != nil {
					Logger.Error("Error getting the open challenges from the blockchain", zap.Error(err))
				} else {
					tCtx := datastore.GetStore().CreateTransaction(ctx)
					db := datastore.GetStore().GetTransaction(tCtx)
					bytesReader := bytes.NewBuffer(retBytes)

					d := json.NewDecoder(bytesReader)
					d.UseNumber()
					errd := d.Decode(&blobberChallenges)

					if errd != nil {
						Logger.Error("Error in unmarshal of the sharder response", zap.Error(errd))
					} else {
						for _, v := range blobberChallenges.Challenges {
							if v == nil || len(v.ChallengeID) == 0 {
								Logger.Info("No challenge entity from the challenge map")
								continue
							}
							if !common.Within(int64(v.Created), int64(time.Duration(config.Configuration.ChallengeResolveFreq).Seconds())) {
								Logger.Info("Challenge is expired")
								continue
							}

							challengeObj := v
							_, err := GetChallengeEntity(tCtx, challengeObj.ChallengeID)

							if errors.Is(err, gorm.ErrRecordNotFound) {
								latestChallenge, err := GetLastChallengeEntity(tCtx)
								if err == nil || errors.Is(err, gorm.ErrRecordNotFound) {
									if (latestChallenge == nil && len(challengeObj.PrevChallengeID) == 0) || latestChallenge.ChallengeID == challengeObj.PrevChallengeID {
										Logger.Info("Adding new challenge found from blockchain", zap.String("challenge", v.ChallengeID))
										challengeObj.Status = Accepted
										if err := challengeObj.Save(tCtx); err != nil {
											Logger.Error("ChallengeEntity_Save", zap.String("challenge_id", challengeObj.ChallengeID), zap.Error(err))
										}
									} else {
										Logger.Error("Challenge chain is not valid")
									}
								}
								//go stats.AddNewChallengeEvent(challengeObj.AllocationID, challengeObj.ID)
							}
						}
					}
					db.Commit()
					tCtx.Done()
				}
				iterInprogress = false
			}
		}
	}
}
