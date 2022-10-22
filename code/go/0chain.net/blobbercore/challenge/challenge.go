package challenge

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"go.uber.org/zap"

	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

type BCChallengeResponse struct {
	BlobberID  string             `json:"blobber_id"`
	Challenges []*ChallengeEntity `json:"challenges"`
}

var lastChallengeTimestamp int

// syncOpenChallenges get challenge from blockchain , and add them in database
func syncOpenChallenges(ctx context.Context) {
	const incrOffset = 20
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[recover]challenge", zap.Any("err", r))
		}
	}()

	offset := 0
	params := make(map[string]string)
	params["blobber"] = node.Self.ID
	params["offset"] = strconv.Itoa(offset)
	params["limit"] = "20"
	if lastChallengeTimestamp > 0 {
		params["from"] = strconv.Itoa(lastChallengeTimestamp)
	}
	start := time.Now()

	var allOpenChallenges []*ChallengeEntity

	var downloadElapsed, jsonElapsed time.Duration

	for {
		var challenges BCChallengeResponse
		var challengeIDs []string
		challenges.Challenges = make([]*ChallengeEntity, 0)
		apiStart := time.Now()
		retBytes, err := transaction.MakeSCRestAPICall(transaction.STORAGE_CONTRACT_ADDRESS, "/openchallenges", params, chain.GetServerChain())
		if err != nil {
			logging.Logger.Error("[challenge]open: ", zap.Error(err))
			break
		}

		downloadElapsed += time.Since(apiStart)

		jsonStart := time.Now()
		bytesReader := bytes.NewBuffer(retBytes)
		d := json.NewDecoder(bytesReader)
		d.UseNumber()
		if err := d.Decode(&challenges); err != nil {
			logging.Logger.Error("[challenge]json: ", zap.String("resp", string(retBytes)), zap.Error(err))
			break
		}
		for _, c := range challenges.Challenges {
			// mark the status as Accepted and send it for processing
			// these challanges will be picked up by challengeProcessor worker
			toProcessChallenge <- TodoChallenge{
				Id:        c.ChallengeID,
				CreatedAt: common.ToTime(c.CreatedAt),
				Status:    Accepted,
			}

			challengeIDs = append(challengeIDs, c.ChallengeID)
			if c.CreatedAt > common.Timestamp(lastChallengeTimestamp) {
				lastChallengeTimestamp = int(c.CreatedAt)
			}
		}
		logging.Logger.Info("challenges_from_chain",
			zap.Int("challenges", len(challenges.Challenges)),
			zap.Strings("challenge_ids", challengeIDs))

		jsonElapsed += time.Since(jsonStart)
		if len(challenges.Challenges) == 0 {
			break
		}
		allOpenChallenges = append(allOpenChallenges, challenges.Challenges...)
		offset += incrOffset
		params["offset"] = strconv.Itoa(offset)
	}

	dbTimeStart := time.Now()
	logging.Logger.Info("Starting saving challenges",
		zap.Int("challenges", len(allOpenChallenges)))

	if len(allOpenChallenges) == 0 {
		return
	}

	logging.Logger.Info("[challenge]elapsed:pull",
		zap.Int("count", len(allOpenChallenges)),
		zap.String("download", downloadElapsed.String()),
		zap.String("json", jsonElapsed.String()),
		zap.String("db", time.Since(dbTimeStart).String()),
		zap.String("time_taken", time.Since(start).String()))

}

func validateOnValidators(id string) {
	startTime := time.Now()

	ctx := datastore.GetStore().CreateTransaction(context.TODO())
	defer ctx.Done()

	c := &ChallengeEntity{}

	tx := datastore.GetStore().GetTransaction(ctx)

	if err := tx.Model(&ChallengeEntity{}).
		Where("challenge_id = ? and status = ?", id, Accepted).
		Find(c).Error; err != nil {

		logging.Logger.Error("[challenge]validate: ",
			zap.Any("challenge_id", id),
			zap.Error(err))

		tx.Rollback()
		return
	}

	createdTime := common.ToTime(c.CreatedAt)
	logging.Logger.Info("[challenge]validate: ",
		zap.String("challenge_id", c.ChallengeID),
		zap.Time("created", createdTime))

	err := c.UnmarshalFields()
	if err != nil {
		logging.Logger.Error("[challenge]validate: ",
			zap.String("challenge_id", c.ChallengeID),
			zap.Time("created", createdTime),
			zap.String("validators", string(c.ValidatorsString)),
			zap.String("lastCommitTxnList", string(c.LastCommitTxnList)),
			zap.String("validationTickets", string(c.ValidationTicketsString)),
			zap.String("ObjectPath", string(c.ObjectPathString)),
			zap.Error(err))
		tx.Rollback()

		c.CancelChallenge(ctx, err)
		return
	}

	if err := c.LoadValidationTickets(ctx); err != nil {
		logging.Logger.Error("[challenge]validate: ",
			zap.Any("challenge_id", c.ChallengeID),
			zap.Time("created", createdTime),
			zap.Error(err))
		tx.Rollback()
		return
	}

	if err := tx.Commit().Error; err != nil {
		logging.Logger.Error("[challenge]validate(Commit): ",
			zap.Any("challenge_id", c.ChallengeID),
			zap.Time("created", createdTime),
			zap.Error(err))
		tx.Rollback()
		return
	}

	completedValidation := time.Now()
	if err := UpdateChallengeTimingCompleteValidation(c.ChallengeID, common.Timestamp(completedValidation.Unix())); err != nil {
		logging.Logger.Error("[challengetiming]validation",
			zap.Any("challenge_id", c.ChallengeID),
			zap.Time("created", createdTime),
			zap.Time("complete_validation", completedValidation),
			zap.Error(err))
	}

	logging.Logger.Info("[challenge]validate: ",
		zap.Any("challenge_id", c.ChallengeID),
		zap.Time("created", createdTime))

	logging.Logger.Info("[challenge]elapsed:validate ",
		zap.String("challenge_id", c.ChallengeID),
		zap.Time("created", createdTime),
		zap.Time("start", startTime),
		zap.String("delay", startTime.Sub(createdTime).String()),
		zap.String("time_taken", time.Since(startTime).String()))

	//nextCommitChallenge <- TodoChallenge{
	//	Id:        c.ChallengeID,
	//	CreatedAt: createdTime,
	//}
	commitOnChain(c, c.ChallengeID)
}

func commitOnChain(c *ChallengeEntity, id string) {

	startTime := time.Now()

	ctx := datastore.GetStore().CreateTransaction(context.TODO())
	defer ctx.Done()

	tx := datastore.GetStore().GetTransaction(ctx)

	if c == nil {
		c := &ChallengeEntity{}

		if err := tx.Model(&ChallengeEntity{}).
			Where("challenge_id = ? and status = ?", id, Processed).
			Find(c).Error; err != nil {

			logging.Logger.Error("[challenge]commit: ",
				zap.Any("challenge_id", id),
				zap.Error(err))

			tx.Rollback()
			return
		}
	}

	createdTime := common.ToTime(c.CreatedAt)
	logging.Logger.Info("[challenge]commit",
		zap.Any("challenge_id", c.ChallengeID),
		zap.Time("created", createdTime),
		zap.Any("openchallenge", c))

	if err := c.UnmarshalFields(); err != nil {
		logging.Logger.Error("[challenge]commit",
			zap.String("challenge_id", c.ChallengeID),
			zap.Time("created", createdTime),
			zap.String("validators", string(c.ValidatorsString)),
			zap.String("lastCommitTxnList", string(c.LastCommitTxnList)),
			zap.String("validationTickets", string(c.ValidationTicketsString)),
			zap.String("ObjectPath", string(c.ObjectPathString)),
			zap.Error(err))
		tx.Rollback()

		c.CancelChallenge(ctx, err)
		return
	}

	elapsedLoad := time.Since(startTime)
	if err := c.CommitChallenge(ctx, false); err != nil {
		logging.Logger.Error("[challenge]commit",
			zap.String("challenge_id", c.ChallengeID),
			zap.Time("created", createdTime),
			zap.Error(err))
		tx.Rollback()
		return
	}

	elapsedCommitOnChain := time.Since(startTime) - elapsedLoad
	if err := tx.Commit().Error; err != nil {
		logging.Logger.Warn("[challenge]commit",
			zap.Any("challenge_id", c.ChallengeID),
			zap.Time("created", createdTime),
			zap.Error(err))
		tx.Rollback()
		return
	}

	elapsedCommitOnDb := time.Since(startTime) - elapsedLoad - elapsedCommitOnChain

	logging.Logger.Info("[challenge]commit",
		zap.Any("challenge_id", c.ChallengeID),
		zap.Time("created", createdTime),
		zap.String("status", c.Status.String()),
		zap.String("txn", c.CommitTxnID))

	logging.Logger.Info("[challenge]elapsed:commit ",
		zap.String("challenge_id", c.ChallengeID),
		zap.Time("created", createdTime),
		zap.Time("start", startTime),
		zap.String("delay", startTime.Sub(createdTime).String()),
		zap.String("load", elapsedLoad.String()),
		zap.String("commit_on_chain", elapsedCommitOnChain.String()),
		zap.String("commit_on_db", elapsedCommitOnDb.String()),
		zap.String("time_taken", time.Since(startTime).String()),
	)

}
