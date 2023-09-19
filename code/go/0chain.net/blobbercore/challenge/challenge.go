package challenge

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"sort"
	"strconv"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"go.uber.org/zap"

	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

type BCChallengeResponse struct {
	BlobberID  string             `json:"blobber_id"`
	Challenges []*ChallengeEntity `json:"challenges"`
}

var lastChallengeRound int64

func syncOpenChallenges(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[recover]challenge", zap.Any("err", r))
		}
	}()

	start := time.Now()

	var downloadElapsed, jsonElapsed time.Duration
	var count int
	for {
		select {
		case <-ctx.Done():
			logging.Logger.Info("sync open challenges main loop ended")
			return
		default:
		}

		params := make(map[string]string)
		params["blobber"] = node.Self.ID

		params["limit"] = "20"
		params["from"] = strconv.FormatInt(lastChallengeRound, 10)

		logging.Logger.Info("[challenge]sync:pull", zap.Any("params", params))

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
		sort.Slice(challenges.Challenges, func(i, j int) bool {
			return challenges.Challenges[i].RoundCreatedAt < challenges.Challenges[j].RoundCreatedAt
		})
		count += len(challenges.Challenges)
		for _, c := range challenges.Challenges {
			challengeIDs = append(challengeIDs, c.ChallengeID)
			if c.RoundCreatedAt >= lastChallengeRound {
				lastChallengeRound = c.RoundCreatedAt
			}
			toProcessChallenge <- c
		}
		logging.Logger.Info("challenges_from_chain",
			zap.Int("challenges", len(challenges.Challenges)),
			zap.Strings("challenge_ids", challengeIDs))

		jsonElapsed += time.Since(jsonStart)
		if len(challenges.Challenges) == 0 {
			break
		}
	}

	dbTimeStart := time.Now()

	logging.Logger.Info("[challenge]elapsed:pull",
		zap.Int("count", count),
		zap.String("download", downloadElapsed.String()),
		zap.String("json", jsonElapsed.String()),
		zap.String("db", time.Since(dbTimeStart).String()),
		zap.String("time_taken", time.Since(start).String()))

}

func validateOnValidators(c *ChallengeEntity) {

	logging.Logger.Info("[challenge]validate: ",
		zap.Any("challenge", c),
		zap.String("challenge_id", c.ChallengeID),
	)

	ctx := datastore.GetStore().CreateTransaction(context.TODO())
	defer ctx.Done()

	tx := datastore.GetStore().GetTransaction(ctx)

	if err := CreateChallengeTiming(c.ChallengeID, c.CreatedAt); err != nil {
		logging.Logger.Error("[challengetiming]add: ",
			zap.String("challenge_id", c.ChallengeID),
			zap.Error(err))
		deleteChallenge(c.RoundCreatedAt)
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
		deleteChallenge(c.RoundCreatedAt)
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

}

func (c *ChallengeEntity) getCommitTransaction() (*transaction.Transaction, error) {
	ctx := datastore.GetStore().CreateTransaction(context.TODO())
	defer ctx.Done()

	tx := datastore.GetStore().GetTransaction(ctx)

	createdTime := common.ToTime(c.CreatedAt)

	logging.Logger.Info("[challenge]verify: ",
		zap.Any("challenge_id", c.ChallengeID),
		zap.Time("created", createdTime))

	currentRound := roundInfo.CurrentRound + int64(float64(roundInfo.LastRoundDiff)*(float64(time.Since(roundInfo.CurrentRoundCaptureTime).Milliseconds())/float64(GetRoundInterval.Milliseconds())))
	logging.Logger.Info("[challenge]commit",
		zap.Any("ChallengeID", c.ChallengeID),
		zap.Any("RoundCreatedAt", c.RoundCreatedAt),
		zap.Any("ChallengeCompletionTime", config.StorageSCConfig.ChallengeCompletionTime),
		zap.Any("currentRound", currentRound),
		zap.Any("roundInfo.LastRoundDiff", roundInfo.LastRoundDiff),
		zap.Any("roundInfo.CurrentRound", roundInfo.CurrentRound),
		zap.Any("roundInfo.CurrentRoundCaptureTime", roundInfo.CurrentRoundCaptureTime),
		zap.Any("time.Since(roundInfo.CurrentRoundCaptureTime).Milliseconds()", time.Since(roundInfo.CurrentRoundCaptureTime).Milliseconds()),
	)

	if currentRound-c.RoundCreatedAt > config.StorageSCConfig.ChallengeCompletionTime {
		c.CancelChallenge(ctx, ErrExpiredCCT)
		if err := tx.Commit().Error; err != nil {
			logging.Logger.Error("[challenge]verify(Commit): ",
				zap.Any("challenge_id", c.ChallengeID),
				zap.Error(err))
		}
		return nil, ErrExpiredCCT
	}

	txn, err := transaction.NewTransactionEntity()
	if err != nil {
		logging.Logger.Error("[challenge]createTxn", zap.Error(err))
		c.CancelChallenge(ctx, err)
		if err := tx.Commit().Error; err != nil {
			logging.Logger.Error("[challenge]verify(Commit): ",
				zap.Any("challenge_id", c.ChallengeID),
				zap.Error(err))
		}
		return nil, err
	}

	sn := &ChallengeResponse{}
	sn.ChallengeID = c.ChallengeID
	for _, vt := range c.ValidationTickets {
		if vt != nil {
			sn.ValidationTickets = append(sn.ValidationTickets, vt)
		}
	}

	err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS, transaction.CHALLENGE_RESPONSE, sn, 0)
	if err != nil {
		logging.Logger.Info("Failed submitting challenge to the mining network", zap.String("err:", err.Error()))
		c.CancelChallenge(ctx, err)
		if err := tx.Commit().Error; err != nil {
			logging.Logger.Error("[challenge]verify(Commit): ",
				zap.Any("challenge_id", c.ChallengeID),
				zap.Error(err))
		}
		return nil, err
	}

	err = UpdateChallengeTimingTxnSubmission(c.ChallengeID, txn.CreationDate)
	if err != nil {
		logging.Logger.Error("[challengetiming]txn_submission",
			zap.Any("challenge_id", c.ChallengeID),
			zap.Time("created", createdTime),
			zap.Any("txn_submission", txn.CreationDate),
			zap.Error(err))
	}

	if err := tx.Commit().Error; err != nil {
		logging.Logger.Error("[challenge]verify(Commit): ",
			zap.Any("challenge_id", c.ChallengeID),
			zap.Error(err))
	}

	return txn, nil
}
