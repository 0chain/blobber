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
			challengeIDs = append(challengeIDs, c.ChallengeID)
			if c.CreatedAt > common.Timestamp(lastChallengeTimestamp) {
				lastChallengeTimestamp = int(c.CreatedAt)
			}
			// make sure that all the fields in ChallengeEntity is populated properly.
			// when a challenge is failed, update it status in DB.
			// but if the blobber paniced when processing a challenge, that challenge will be lost forever
			c.Status = Accepted
			err := c.MarshallFields()
			if err != nil {
				logging.Logger.Error("[challenge]open: ", zap.Error(err))
				break
			}
			challengeEntityChan <- c
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

func validateOnValidators(c *ChallengeEntity) {
	startTime := time.Now()

	ctx := datastore.GetStore().CreateTransaction(context.TODO())
	defer ctx.Done()

	// TODO: what if the transaction didn't start before call tx.Rollback() and tx.Commit()?
	tx := datastore.GetStore().GetTransaction(ctx)

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

	// after loading validation tickets, marks the challenge as processed
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

	// 2mins delay is too much
	// "2022-10-25T13:03:42.684Z\tINFO\tchallenge/challenge.go:179\t[challenge]elapsed:validate \t{\"challenge_id\": \"3f8be35370471e1e3551e6d8e64f82554078fbb8c426c2d9c6bdcb439913bf1b\", \"created\": \"2022-10-25T12:56:57.000Z\", \"start\": \"2022-10-25T12:59:07.998Z\", \"delay\": \"2m10.998355267s\", \"time_taken\": \"4m34.685931162s\"}\n"
	logging.Logger.Info("[challenge]elapsed:validate ",
		zap.String("challenge_id", c.ChallengeID),
		zap.Time("created", createdTime),
		zap.Time("start", startTime),
		zap.String("delay", startTime.Sub(createdTime).String()),
		zap.String("time_taken", time.Since(startTime).String()))

	// when the challenge is processed, commit it on chain
	commitOnChain(c)
}

func commitOnChain(c *ChallengeEntity) {

	startTime := time.Now()

	ctx := datastore.GetStore().CreateTransaction(context.TODO())
	defer ctx.Done()

	// tx := datastore.GetStore().GetTransaction(ctx)

	// if c == nil {
	// 	c := &ChallengeEntity{}

	// 	if err := tx.Model(&ChallengeEntity{}).
	// 		Where("challenge_id = ? and status = ?", id, Processed).
	// 		Find(c).Error; err != nil {

	// 		logging.Logger.Error("[challenge]commit: ",
	// 			zap.Any("challenge_id", id),
	// 			zap.Error(err))

	// 		tx.Rollback()
	// 		return
	// 	}
	// }

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
		// tx.Rollback()

		c.CancelChallenge(ctx, err)
		return
	}

	elapsedLoad := time.Since(startTime)
	if err := c.CommitChallenge(ctx, false); err != nil {
		logging.Logger.Error("[challenge]commit",
			zap.String("challenge_id", c.ChallengeID),
			zap.Time("created", createdTime),
			zap.Error(err))
		// tx.Rollback()
		return
	}

	elapsedCommitOnChain := time.Since(startTime) - elapsedLoad
	// if err := tx.Commit().Error; err != nil {
	// 	logging.Logger.Warn("[challenge]commit",
	// 		zap.Any("challenge_id", c.ChallengeID),
	// 		zap.Time("created", createdTime),
	// 		zap.Error(err))
	// 	tx.Rollback()
	// 	return
	// }

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
