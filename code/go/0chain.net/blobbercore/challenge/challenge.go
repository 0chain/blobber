package challenge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"github.com/remeh/sizedwaitgroup"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

type BCChallengeResponse struct {
	BlobberID  string             `json:"blobber_id"`
	Challenges []*ChallengeEntity `json:"challenges"`
}

// syncOpenChallenges get challenge from blockchain , and add them in database
func syncOpenChallenges(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[recover]challenge", zap.Any("err", r))
		}
	}()

	params := make(map[string]string)
	params["blobber"] = node.Self.ID

	var blobberChallenges BCChallengeResponse
	blobberChallenges.Challenges = make([]*ChallengeEntity, 0)
	retBytes, err := transaction.MakeSCRestAPICall(transaction.STORAGE_CONTRACT_ADDRESS, "/openchallenges", params, chain.GetServerChain())

	if err != nil {
		logging.Logger.Error("[challenge]open: ", zap.Error(err))
	} else {
		bytesReader := bytes.NewBuffer(retBytes)

		d := json.NewDecoder(bytesReader)
		d.UseNumber()
		errd := d.Decode(&blobberChallenges)

		if errd != nil {
			logging.Logger.Error("[challenge]json: ", zap.Error(errd))
		} else {
			for _, challengeObj := range blobberChallenges.Challenges {
				if challengeObj == nil || challengeObj.ChallengeID == "" {
					logging.Logger.Info("[challenge]open: No challenge entity from the challenge map")
					continue
				}

				tx := datastore.GetStore().CreateTransaction(ctx)
				db := datastore.GetStore().GetTransaction(tx)
				_, err := GetChallengeEntity(tx, challengeObj.ChallengeID)

				// challenge is not synced in db yet
				if errors.Is(err, gorm.ErrRecordNotFound) {
					latestChallenge, err := GetLastChallengeEntity(tx)

					if err != nil {
						if !errors.Is(err, gorm.ErrRecordNotFound) {
							logging.Logger.Error("[challenge]db: ", zap.Error(err))
							continue
						}
					}

					isFirstChallengeInDatabase := challengeObj.PrevChallengeID == "" || latestChallenge == nil
					isNextChallengeOnChain := latestChallenge == nil || latestChallenge.ChallengeID == challengeObj.PrevChallengeID

					if isFirstChallengeInDatabase || isNextChallengeOnChain {
						logging.Logger.Info("[challenge]add: ", zap.String("challenge_id", challengeObj.ChallengeID))
						challengeObj.Status = Accepted
						challengeObj.CreatedAt = common.ToTime(challengeObj.Created)
						challengeObj.UpdatedAt = challengeObj.CreatedAt

						if err := challengeObj.Save(tx); err != nil {
							logging.Logger.Error("[challenge]db: ", zap.String("challenge_id", challengeObj.ChallengeID), zap.Error(err))
						}
					} else {
						logging.Logger.Error("[challenge]Challenge chain is not valid")
					}
				}
				db.Commit()
				tx.Done()
			}
		}
	}
}

// processAccepted read accepted challenge from db, and send them to validator to pass challenge
func processAccepted(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[recover]challenge", zap.Any("err", r))
		}
	}()
	rctx := datastore.GetStore().CreateTransaction(ctx)
	db := datastore.GetStore().GetTransaction(rctx)
	openchallenges := make([]*ChallengeEntity, 0)
	db.Where(ChallengeEntity{Status: Accepted}).Find(&openchallenges)
	if len(openchallenges) > 0 {
		swg := sizedwaitgroup.New(config.Configuration.ChallengeResolveNumWorkers)
		for _, openchallenge := range openchallenges {
			logging.Logger.Info("[challenge]process: ", zap.String("challenge_id", openchallenge.ChallengeID))
			err := openchallenge.UnmarshalFields()
			if err != nil {
				logging.Logger.Error("[challenge]json: ", zap.Error(err))
				continue
			}
			swg.Add()
			go func(redeemCtx context.Context, challengeEntity *ChallengeEntity) {
				defer swg.Done()
				redeemCtx = datastore.GetStore().CreateTransaction(redeemCtx)
				defer redeemCtx.Done()
				err := loadValidationTickets(redeemCtx, challengeEntity)
				if err != nil {
					logging.Logger.Error("[challenge]validate: ", zap.Any("challenge_id", challengeEntity.ChallengeID), zap.Error(err))
					return
				}
				db := datastore.GetStore().GetTransaction(redeemCtx)
				err = db.Commit().Error
				if err != nil {
					logging.Logger.Error("[challenge]db: ", zap.Any("challenge_id", challengeEntity.ChallengeID), zap.Error(err))
					return
				}
			}(ctx, openchallenge)
		}
		swg.Wait()
	}
	db.Rollback()
	rctx.Done()
}

// loadValidationTickets load validation tickets for challenge
func loadValidationTickets(ctx context.Context, challengeObj *ChallengeEntity) error {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[recover]LoadValidationTickets", zap.Any("err", r))
		}
	}()

	err := challengeObj.LoadValidationTickets(ctx)
	if err != nil {
		logging.Logger.Error("[challenge]load: ", zap.String("challenge_id", challengeObj.ChallengeID), zap.Error(err))
	}

	return err
}

func commitProcessed(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[recover]challenge", zap.Any("err", r))
		}
	}()

	rctx := datastore.GetStore().CreateTransaction(ctx)
	db := datastore.GetStore().GetTransaction(rctx)
	openchallenges := make([]*ChallengeEntity, 0)

	db.Where(ChallengeEntity{Status: Processed}).
		Order("sequence").
		Find(&openchallenges)

	for _, openchallenge := range openchallenges {
		logging.Logger.Info("Attempting to commit challenge", zap.Any("challenge_id", openchallenge.ChallengeID), zap.Any("openchallenge", openchallenge))
		if err := openchallenge.UnmarshalFields(); err != nil {
			logging.Logger.Error("ChallengeEntity_UnmarshalFields", zap.String("challenge_id", openchallenge.ChallengeID), zap.Error(err))
		}

		redeemCtx := datastore.GetStore().CreateTransaction(ctx)
		err := openchallenge.CommitChallenge(redeemCtx, false)
		if err != nil {
			logging.Logger.Error("Error committing to blockchain",
				zap.Error(err),
				zap.String("challenge_id", openchallenge.ChallengeID))
		}

		db := datastore.GetStore().GetTransaction(redeemCtx)
		db.Commit()
		if err == nil && openchallenge.Status == Committed {
			logging.Logger.Info("Challenge has been submitted to blockchain",
				zap.Any("id", openchallenge.ChallengeID),
				zap.String("txn", openchallenge.CommitTxnID))
		} else {
			logging.Logger.Info("Challenge was not committed", zap.Any("challenge_id", openchallenge.ChallengeID))
			break
		}
	}

	db.Rollback()
	rctx.Done()
}
