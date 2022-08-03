package challenge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/cache"
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

var cMap = cache.NewLRUCache(2000)

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

	startTime := time.Now()
	retBytes, err := transaction.MakeSCRestAPICall(transaction.STORAGE_CONTRACT_ADDRESS, "/openchallenges", params, chain.GetServerChain())

	if err != nil {
		logging.Logger.Error("[challenge]open: ", zap.Error(err))
		return
	}

	downloadElapsed := time.Since(startTime)

	bytesReader := bytes.NewBuffer(retBytes)
	d := json.NewDecoder(bytesReader)
	d.UseNumber()
	if err := d.Decode(&blobberChallenges); err != nil {
		logging.Logger.Error("[challenge]json: ", zap.String("resp", string(retBytes)), zap.Error(err))
		return
	}

	jsonElapsed := time.Since(startTime)

	logging.Logger.Info("[challenge]elapsed:pull",
		zap.Int("count", len(blobberChallenges.Challenges)),
		zap.String("download", downloadElapsed.String()),
		zap.String("json", (jsonElapsed-downloadElapsed).String()))

	for _, challengeObj := range blobberChallenges.Challenges {

		if challengeObj == nil || challengeObj.ChallengeID == "" {
			logging.Logger.Info("[challenge]open: No challenge entity from the challenge map")
			continue
		}

		saveNewChallenge(challengeObj, ctx)
	}

}

func saveNewChallenge(c *ChallengeEntity, ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[recover]add_challenge", zap.Any("err", r))
		}
	}()

	startTime := time.Now()

	if _, err := cMap.Get(c.ChallengeID); err == nil {
		return
	}

	db := datastore.GetStore().GetDB()
	if status := getStatus(db, c.ChallengeID); status != nil {
		cMap.Add(c.ChallengeID, *status) //nolint
		return
	}

	c.Status = Accepted
	c.CreatedAt = common.ToTime(c.Created)
	c.UpdatedAt = c.CreatedAt

	logging.Logger.Info("[challenge]add: ",
		zap.String("challenge_id", c.ChallengeID),
		zap.Time("created", c.CreatedAt))

	if err := db.Transaction(func(tx *gorm.DB) error {
		return c.SaveWith(tx)
	}); err != nil {
		logging.Logger.Error("[challenge]add: ",
			zap.String("challenge_id", c.ChallengeID),
			zap.Time("created", c.CreatedAt),
			zap.Error(err))

		return
	}

	cMap.Add(c.ChallengeID, Accepted) //nolint

	logging.Logger.Info("[challenge]elapsed:add ",
		zap.String("challenge_id", c.ChallengeID),
		zap.Time("created", c.CreatedAt),
		zap.Time("start", startTime),
		zap.String("delay", startTime.Sub(c.CreatedAt).String()),
		zap.String("save", time.Since(startTime).String()))

}

// processAccepted read accepted challenge from db, and send them to validator to pass challenge
func processAccepted(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[recover]challenge", zap.Any("err", r))
		}
	}()

	db := datastore.GetStore().GetDB()

	rows, err := db.Model(&ChallengeEntity{}).
		Where("status = ?", Accepted).
		Select("challenge_id", "created_at").Rows()

	if err != nil {
		logging.Logger.Error("[challenge]process: ",
			zap.Error(err))
		return
	}

	startTime := time.Now()
	swg := sizedwaitgroup.New(config.Configuration.ChallengeResolveNumWorkers)
	count := 0
	for rows.Next() {
		count++
		now := time.Now()

		var challengeID string
		var createdAt time.Time

		err := rows.Scan(&challengeID, &createdAt)
		if err != nil {
			logging.Logger.Error("[challenge]process: ",
				zap.Error(err))
			continue
		}

		if time.Since(createdAt) > config.Configuration.ChallengeCompletionTime {

			db.Model(&ChallengeEntity{}).
				Where("challenge_id =? and status =? ", challengeID, Accepted).
				Updates(map[string]interface{}{
					"status":         Cancelled,
					"result":         ChallengeFailure,
					"status_message": fmt.Sprintf("created: %s, start: %s , delay: %s, cct: %s", createdAt, now, now.Sub(createdAt).String(), config.Configuration.ChallengeCompletionTime.String()),
				})

			logging.Logger.Error("[challenge]process: timeout ",
				zap.Any("challenge_id", challengeID),
				zap.Time("created", createdAt),
				zap.Time("start", now),
				zap.String("delay", now.Sub(createdAt).String()),
				zap.String("cct", config.Configuration.ChallengeCompletionTime.String()),
				zap.Error(err))
			continue
		}

		swg.Add()
		go func(id string) {
			defer swg.Done()
			validateChallenge(id)
		}(challengeID)

	}

	swg.Wait()

	logging.Logger.Info("[challenge]elapsed:process ",
		zap.Int("count", count),
		zap.String("save", time.Since(startTime).String()))

}

func validateChallenge(id string) {
	startTime := time.Now()

	ctx := datastore.GetStore().CreateTransaction(context.TODO())
	defer ctx.Done()

	var c *ChallengeEntity

	db := datastore.GetStore().GetTransaction(ctx)

	if err := db.Model(&ChallengeEntity{}).
		Where("challenge_id = ? and status = ?", id, Accepted).
		Find(c).Error; err != nil {

		logging.Logger.Error("[challenge]validate: ",
			zap.Any("challenge_id", id),
			zap.Error(err))

		db.Rollback()
		return
	}

	logging.Logger.Info("[challenge]validate: ",
		zap.String("challenge_id", c.ChallengeID),
		zap.Time("created", c.CreatedAt))

	err := c.UnmarshalFields()
	if err != nil {
		logging.Logger.Error("[challenge]validate: ",
			zap.String("challenge_id", c.ChallengeID),
			zap.Time("created", c.CreatedAt),
			zap.String("validators", string(c.ValidatorsString)),
			zap.String("lastCommitTxnList", string(c.LastCommitTxnList)),
			zap.String("validationTickets", string(c.ValidationTicketsString)),
			zap.String("ObjectPath", string(c.ObjectPathString)),
			zap.Error(err))
		return
	}

	if err := c.LoadValidationTickets(ctx); err != nil {
		logging.Logger.Error("[challenge]validate: ",
			zap.Any("challenge_id", c.ChallengeID),
			zap.Time("created", c.CreatedAt),
			zap.Error(err))
		db.Rollback()
		return
	}

	if err := db.Commit().Error; err != nil {
		logging.Logger.Error("[challenge]validate(Commit): ",
			zap.Any("challenge_id", c.ChallengeID),
			zap.Time("created", c.CreatedAt),
			zap.Error(err))
		db.Rollback()
		return
	}

	logging.Logger.Info("[challenge]validate: ",
		zap.Any("challenge_id", c.ChallengeID),
		zap.Time("created", c.CreatedAt))

	logging.Logger.Info("[challenge]elapsed:validate ",
		zap.String("challenge_id", c.ChallengeID),
		zap.Time("created", c.CreatedAt),
		zap.Time("start", startTime),
		zap.String("delay", startTime.Sub(c.CreatedAt).String()),
		zap.String("save", time.Since(startTime).String()))
}

func commitProcessed(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[recover]challenge", zap.Any("err", r))
		}
	}()

	db := datastore.GetStore().GetDB()
	count := 0

	rows, err := db.Model(&ChallengeEntity{}).
		Where("status = ?", Processed).
		Select("challenge_id", "created_at").Rows()

	if err != nil {
		logging.Logger.Error("[challenge]validate: ",
			zap.Error(err))
		return
	}

	startTime := time.Now()
	swg := sizedwaitgroup.New(config.Configuration.ChallengeResolveNumWorkers)

	for rows.Next() {
		count++
		now := time.Now()

		var challengeID string
		var createdAt time.Time

		err := rows.Scan(&challengeID, &createdAt)
		if err != nil {
			logging.Logger.Error("[challenge]commit: ",
				zap.Error(err))
			continue
		}

		if time.Since(createdAt) > config.Configuration.ChallengeCompletionTime {

			db.Model(&ChallengeEntity{}).
				Where("challenge_id =? and status =? ", challengeID, Accepted).
				Updates(map[string]interface{}{
					"status":         Cancelled,
					"result":         ChallengeFailure,
					"status_message": fmt.Sprintf("created: %s, start: %s , delay: %s, cct: %s", createdAt, now, now.Sub(createdAt).String(), config.Configuration.ChallengeCompletionTime.String()),
				})

			logging.Logger.Error("[challenge]commit: timeout ",
				zap.Any("challenge_id", challengeID),
				zap.Time("created", createdAt),
				zap.Time("start", now),
				zap.String("delay", now.Sub(createdAt).String()),
				zap.String("cct", config.Configuration.ChallengeCompletionTime.String()),
				zap.Error(err))
			continue
		}

		swg.Add()
		go func(id string) {
			defer swg.Done()
			commitChallenge(id)
		}(challengeID)

	}

	swg.Wait()

	logging.Logger.Info("[challenge]elapsed:commit ",
		zap.Int("count", count),
		zap.String("save", time.Since(startTime).String()))
}

func commitChallenge(id string) {

	ctx := datastore.GetStore().CreateTransaction(context.TODO())
	defer ctx.Done()

	db := datastore.GetStore().GetTransaction(ctx)

	var c *ChallengeEntity

	if err := db.Model(&ChallengeEntity{}).
		Where("challenge_id = ? and status = ?", id, Processed).
		Find(c).Error; err != nil {

		logging.Logger.Error("[challenge]commit: ",
			zap.Any("challenge_id", id),
			zap.Error(err))

		db.Rollback()
		return
	}

	startTime := time.Now()

	logging.Logger.Info("[challenge]commit",
		zap.Any("challenge_id", c.ChallengeID),
		zap.Time("created", c.CreatedAt),
		zap.Any("openchallenge", c))

	if err := c.UnmarshalFields(); err != nil {
		logging.Logger.Error("[challenge]commit",
			zap.String("challenge_id", c.ChallengeID),
			zap.Time("created", c.CreatedAt),
			zap.String("validators", string(c.ValidatorsString)),
			zap.String("lastCommitTxnList", string(c.LastCommitTxnList)),
			zap.String("validationTickets", string(c.ValidationTicketsString)),
			zap.String("ObjectPath", string(c.ObjectPathString)),
			zap.Error(err))
		return
	}

	if err := c.CommitChallenge(ctx, false); err != nil {
		logging.Logger.Error("[challenge]commit",
			zap.String("challenge_id", c.ChallengeID),
			zap.Time("created", c.CreatedAt),
			zap.Error(err))
		db.Rollback()
		return
	}

	if err := db.Commit().Error; err != nil {
		logging.Logger.Warn("[challenge]commit",
			zap.Any("challenge_id", c.ChallengeID),
			zap.Time("created", c.CreatedAt),
			zap.Error(err))
		db.Rollback()
		return
	}

	logging.Logger.Info("[challenge]commit",
		zap.Any("challenge_id", c.ChallengeID),
		zap.Time("created", c.CreatedAt),
		zap.String("status", c.Status.String()),
		zap.String("txn", c.CommitTxnID))

	logging.Logger.Info("[challenge]elapsed:commit ",
		zap.String("challenge_id", c.ChallengeID),
		zap.Time("created", c.CreatedAt),
		zap.Time("start", startTime),
		zap.String("delay", startTime.Sub(c.CreatedAt).String()),
		zap.String("save", time.Since(startTime).String()))

	go cMap.Delete(c.ChallengeID) //nolint

}
