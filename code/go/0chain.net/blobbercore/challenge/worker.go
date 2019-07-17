package challenge

import (
	
	"context"
	"bytes"
	"encoding/json"
	"time"

	"0chain.net/core/lock"
	"0chain.net/core/node"
	"0chain.net/core/transaction"
	"0chain.net/core/chain"
	."0chain.net/core/logging"
	"0chain.net/blobbercore/datastore"
	"0chain.net/blobbercore/config"

	"github.com/remeh/sizedwaitgroup"
	"go.uber.org/zap"
	"github.com/jinzhu/gorm"
	
)

type BCChallengeResponse struct {
	BlobberID  string             `json:"blobber_id"`
	Challenges []*ChallengeEntity `json:"challenges"`
}

func SetupWorkers(ctx context.Context) {
	go FindChallenges(ctx)
	go SubmitProcessedChallenges(ctx)
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
	for true {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			rctx := datastore.GetStore().CreateTransaction(ctx)
			db := datastore.GetStore().GetTransaction(rctx)
			//lastChallengeRedeemed := &ChallengeEntity{}
			rows, _ := db.Table("challenges").Select("commit_txn_id, sequence"). Where(ChallengeEntity{Status: Committed}).Order("sequence desc").Limit(1).Rows()
			lastSeq := 0
			lastCommitTxn := ""
			for rows.Next() {
				rows.Scan(&lastCommitTxn, &lastSeq)
			}
			
			openchallenges := make([]*ChallengeEntity, 0)
			db.Where(ChallengeEntity{Status: Processed}).Where("sequence > ?", lastSeq).Order("sequence").Find(&openchallenges)
			if len(openchallenges) > 0 {
				for _, openchallenge := range openchallenges {
					openchallenge.UnmarshalFields()
					mutex := lock.GetMutex(openchallenge.TableName(), openchallenge.ChallengeID)
					mutex.Lock()
					redeemCtx := datastore.GetStore().CreateTransaction(ctx)
					err := openchallenge.CommitChallenge(redeemCtx, false)
					if err != nil {
						Logger.Error("Error committing to blockchain", zap.Error(err), zap.String("challenge_id", openchallenge.ChallengeID))
					}
					mutex.Unlock()
					db := datastore.GetStore().GetTransaction(redeemCtx)
					db.Commit()
					if err == nil && openchallenge.Status == Committed {
						Logger.Info("Challenge has been submitted to blockchain", zap.Any("id", openchallenge.ChallengeID), zap.String("txn", openchallenge.CommitTxnID))
					}
				}
			}
			db.Rollback()
			rctx.Done()

			rctx = datastore.GetStore().CreateTransaction(ctx)
			db = datastore.GetStore().GetTransaction(rctx)
			toBeVerifiedChallenges := make([]*ChallengeEntity, 0)
			//commit challenges on local state for all challenges that have missed the commit txn from blockchain
			db.Debug().Where(ChallengeEntity{Status: Processed}).Where("sequence < ?", lastSeq).Find(&toBeVerifiedChallenges)
			for _, toBeVerifiedChallenge := range toBeVerifiedChallenges {
				toBeVerifiedChallenge.UnmarshalFields()
				mutex := lock.GetMutex(toBeVerifiedChallenge.TableName(), toBeVerifiedChallenge.ChallengeID)
				mutex.Lock()
				redeemCtx := datastore.GetStore().CreateTransaction(ctx)
				err := toBeVerifiedChallenge.CommitChallenge(redeemCtx, true)
				if err != nil {
					Logger.Error("Error committing to blockchain", zap.Error(err), zap.String("challenge_id", toBeVerifiedChallenge.ChallengeID))
				}
				mutex.Unlock()
				db := datastore.GetStore().GetTransaction(redeemCtx)
				db.Commit()
				if err == nil && toBeVerifiedChallenge.Status == Committed {
					Logger.Info("Challenge has been submitted to blockchain", zap.Any("id", toBeVerifiedChallenge.ChallengeID), zap.String("txn", toBeVerifiedChallenge.CommitTxnID))
				}
			}
			db.Rollback()
			rctx.Done()
			
		}
		time.Sleep(time.Duration(config.Configuration.ChallengeResolveFreq) * time.Second)
	}
	

	// if challengeObj.ObjectPath != nil && challengeObj.Status == Committed && challengeObj.ObjectPath.FileBlockNum > 0 {
	// 	//stats.FileChallenged(challengeObj.AllocationID, challengeObj.ObjectPath.Meta["path"].(string), challengeObj.CommitTxnID)
	// 	if challengeObj.Result == ChallengeSuccess {
	// 		go stats.AddChallengeRedeemedEvent(challengeObj.AllocationID, challengeObj.ID, stats.SUCCESS, stats.REDEEMSUCCESS, challengeObj.ObjectPath.Meta["path"].(string), challengeObj.CommitTxnID)
	// 	} else if challengeObj.Result == ChallengeFailure {
	// 		go stats.AddChallengeRedeemedEvent(challengeObj.AllocationID, challengeObj.ID, stats.FAILED, stats.REDEEMSUCCESS, challengeObj.ObjectPath.Meta["path"].(string), challengeObj.CommitTxnID)
	// 	}

	// } else if challengeObj.ObjectPath != nil && challengeObj.Status != Committed && challengeObj.ObjectPath.FileBlockNum > 0 && challengeObj.Retries >= config.Configuration.ChallengeMaxRetires {
	// 	if challengeObj.Result == ChallengeSuccess {
	// 		go stats.AddChallengeRedeemedEvent(challengeObj.AllocationID, challengeObj.ID, stats.SUCCESS, stats.REDEEMERROR, challengeObj.ObjectPath.Meta["path"].(string), challengeObj.CommitTxnID)
	// 	} else if challengeObj.Result == ChallengeFailure {
	// 		go stats.AddChallengeRedeemedEvent(challengeObj.AllocationID, challengeObj.ID, stats.FAILED, stats.REDEEMERROR, challengeObj.ObjectPath.Meta["path"].(string), challengeObj.CommitTxnID)
	// 	}
	// }
	
	return nil
}

var iterInprogress = false

func FindChallenges(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.Configuration.ChallengeResolveFreq) * time.Second)
	for true {
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
							GetValidationTickets(redeemCtx, challengeEntity)
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
							challengeObj := v
							_, err := GetChallengeEntity(tCtx, challengeObj.ChallengeID)
							if gorm.IsRecordNotFoundError(err) {
								latestChallenge, err := GetLastChallengeEntity(tCtx)
								if err == nil || gorm.IsRecordNotFoundError(err) {
									if (latestChallenge == nil && len(challengeObj.PrevChallengeID) == 0) ||  latestChallenge.ChallengeID == challengeObj.PrevChallengeID {
										Logger.Info("Adding new challenge found from blockchain", zap.String("challenge", v.ChallengeID))
										challengeObj.Status = Accepted
										challengeObj.Save(tCtx)
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
