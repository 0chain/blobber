package challenge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

type BCChallengeResponse struct {
	BlobberID  string             `json:"blobber_id"`
	Challenges []*ChallengeEntity `json:"challenges"`
}

// syncChallenges get challenge from blockchain , and add them in database
func syncChallenges(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("Error getting the open challenges from the blockchain", zap.Any("err", r))
		}
	}()

	params := make(map[string]string)
	params["blobber"] = node.Self.ID

	var blobberChallenges BCChallengeResponse
	blobberChallenges.Challenges = make([]*ChallengeEntity, 0)
	retBytes, err := transaction.MakeSCRestAPICall(transaction.STORAGE_CONTRACT_ADDRESS, "/openchallenges", params, chain.GetServerChain())

	if err != nil {
		logging.Logger.Error("Error getting the open challenges from the blockchain", zap.Error(err))
	} else {

		bytesReader := bytes.NewBuffer(retBytes)

		d := json.NewDecoder(bytesReader)
		d.UseNumber()
		errd := d.Decode(&blobberChallenges)

		if errd != nil {
			logging.Logger.Error("Error in unmarshal of the sharder response", zap.Error(errd))
		} else {
			for _, challengeObj := range blobberChallenges.Challenges {
				if challengeObj == nil || len(challengeObj.ChallengeID) == 0 {
					logging.Logger.Info("No challenge entity from the challenge map")
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
							logging.Logger.Info("Error in load challenge entity from database ", zap.Error(err))
							continue
						}
					}

					isFirstChallengeInDatabase := len(challengeObj.PrevChallengeID) == 0 || latestChallenge == nil
					isNextChallengeOnChain := latestChallenge.ChallengeID == challengeObj.PrevChallengeID

					if isFirstChallengeInDatabase || isNextChallengeOnChain {
						logging.Logger.Info("Adding new challenge found from blockchain", zap.String("challenge", challengeObj.ChallengeID))
						challengeObj.Status = Accepted
						if err := challengeObj.Save(tx); err != nil {
							logging.Logger.Error("ChallengeEntity_Save", zap.String("challenge_id", challengeObj.ChallengeID), zap.Error(err))
						}
					} else {
						logging.Logger.Error("Challenge chain is not valid")
					}

				}
				db.Commit()
				tx.Done()
			}
		}

	}
}
