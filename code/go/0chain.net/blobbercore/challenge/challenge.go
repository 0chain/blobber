package challenge

import (
	"bytes"
	"context"
	"encoding/json"

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
		return
	}

	bytesReader := bytes.NewBuffer(retBytes)
	d := json.NewDecoder(bytesReader)
	d.UseNumber()
	if err := d.Decode(&blobberChallenges); err != nil {
		logging.Logger.Error("[challenge]json: ", zap.Error(err))
		return
	}

	for _, challengeObj := range blobberChallenges.Challenges {
		if challengeObj == nil || challengeObj.ChallengeID == "" {
			logging.Logger.Info("[challenge]open: No challenge entity from the challenge map")
			continue
		}
		saveNewChallenge(challengeObj, ctx)
	}

}

func saveNewChallenge(nextChallenge *ChallengeEntity, ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[recover]challenge", zap.Any("err", r))
		}
	}()

	db := datastore.GetStore().GetDB()
	if Exists(db, nextChallenge.ChallengeID) {
		return
	}

	lastChallengeID, err := getLastChallengeID(db)

	if err != nil {
		logging.Logger.Error("[challenge]db: ", zap.Error(err))
		return
	}

	isValid := nextChallenge.PrevChallengeID == "" || lastChallengeID == nextChallenge.PrevChallengeID

	// it is not First and Next challenge
	if !isValid {
		logging.Logger.Error("[challenge]Challenge chain is not valid")
		return
	}

	logging.Logger.Info("[challenge]add: ", zap.String("challenge_id", nextChallenge.ChallengeID))
	nextChallenge.Status = Accepted
	nextChallenge.CreatedAt = common.ToTime(nextChallenge.Created)
	nextChallenge.UpdatedAt = nextChallenge.CreatedAt

	err = db.Transaction(func(tx *gorm.DB) error {
		return nextChallenge.SaveWith(tx)
	})
	if err != nil {
		logging.Logger.Error("[challenge]db: ", zap.String("challenge_id", nextChallenge.ChallengeID), zap.Error(err))
		return
	}

	go func() {
		waitToProcess <- nextChallenge
	}()

}

// processAccepted read accepted challenge from db, and send them to validator to pass challenge
func processAccepted(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[recover]challenge", zap.Any("err", r))
		}
	}()

	db := datastore.GetStore().GetDB()
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
			go validateChallenge(&swg, openchallenge)
		}
		swg.Wait()
	}
}

func validateChallenge(swg *sizedwaitgroup.SizedWaitGroup, challengeObj *ChallengeEntity) {
	defer func() {
		if swg != nil {
			swg.Done()
		}
	}()

	ctx := datastore.GetStore().CreateTransaction(context.TODO())
	defer ctx.Done()

	db := datastore.GetStore().GetTransaction(ctx)
	if err := challengeObj.LoadValidationTickets(ctx); err != nil {
		logging.Logger.Error("[challenge]validate: ", zap.Any("challenge_id", challengeObj.ChallengeID), zap.Error(err))
		db.Rollback()
		return
	}

	err := db.Commit().Error
	if err != nil {
		logging.Logger.Error("[challenge]db: ", zap.Any("challenge_id", challengeObj.ChallengeID), zap.Error(err))
		db.Rollback()
		return
	}

	go func() {
		waitToCommit <- challengeObj
	}()
}

func commitProcessed(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[recover]challenge", zap.Any("err", r))
		}
	}()

	db := datastore.GetStore().GetDB()
	var challenges []*ChallengeEntity

	db.Where(ChallengeEntity{Status: Processed}).
		Order("sequence").
		Find(&challenges)

	if len(challenges) > 0 {
		swg := sizedwaitgroup.New(config.Configuration.ChallengeResolveNumWorkers)
		for _, challenge := range challenges {
			c := challenge
			swg.Add()
			go func() {
				defer swg.Done()
				commitChallenge(c)
			}()
		}
		swg.Wait()
	}
}

func commitChallenge(openchallenge *ChallengeEntity) {
	logging.Logger.Info("Attempting to commit challenge", zap.Any("challenge_id", openchallenge.ChallengeID), zap.Any("openchallenge", openchallenge))
	if err := openchallenge.UnmarshalFields(); err != nil {
		logging.Logger.Error("ChallengeEntity_UnmarshalFields", zap.String("challenge_id", openchallenge.ChallengeID), zap.Error(err))
	}

	ctx := datastore.GetStore().CreateTransaction(context.TODO())
	defer ctx.Done()

	db := datastore.GetStore().GetTransaction(ctx)

	if err := openchallenge.CommitChallenge(ctx, false); err != nil {
		logging.Logger.Error("Error committing to blockchain",
			zap.Error(err),
			zap.String("challenge_id", openchallenge.ChallengeID))
		db.Rollback()
		return
	}

	if err := db.Commit(); err != nil {
		logging.Logger.Info("Challenge was not committed", zap.Any("challenge_id", openchallenge.ChallengeID))
		db.Rollback()
		return
	}

	if openchallenge.Status == Committed {
		logging.Logger.Info("Challenge has been submitted to blockchain",
			zap.Any("id", openchallenge.ChallengeID),
			zap.String("txn", openchallenge.CommitTxnID))
	}
}
