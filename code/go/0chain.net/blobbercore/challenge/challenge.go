package challenge

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/cache"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
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

	for _, challengeObj := range blobberChallenges.Challenges {

		if challengeObj == nil || challengeObj.ChallengeID == "" {
			logging.Logger.Info("[challenge]open: No challenge entity from the challenge map")
			continue
		}

		saveNewChallenge(challengeObj, ctx)
	}

	logging.Logger.Info("[challenge]elapsed:pull",
		zap.Int("count", len(blobberChallenges.Challenges)),
		zap.String("download", downloadElapsed.String()),
		zap.String("json", (jsonElapsed-downloadElapsed).String()),
		zap.String("db", (time.Since(startTime)-downloadElapsed-jsonElapsed).String()))

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
	createdTime := common.ToTime(c.CreatedAt)

	logging.Logger.Info("[challenge]add: ",
		zap.String("challenge_id", c.ChallengeID),
		zap.Time("created", createdTime))

	if err := db.Transaction(func(tx *gorm.DB) error {
		return c.SaveWith(tx)
	}); err != nil {
		logging.Logger.Error("[challenge]add: ",
			zap.String("challenge_id", c.ChallengeID),
			zap.Time("created", createdTime),
			zap.Error(err))

		return
	}

	cMap.Add(c.ChallengeID, Accepted) //nolint

	logging.Logger.Info("[challenge]elapsed:add ",
		zap.String("challenge_id", c.ChallengeID),
		zap.Time("created", createdTime),
		zap.Time("start", startTime),
		zap.String("delay", startTime.Sub(createdTime).String()),
		zap.String("save", time.Since(startTime).String()))

	nextTodoChallenge <- TodoChallenge{
		Id:        c.ChallengeID,
		CreatedAt: common.ToTime(c.CreatedAt),
		Status:    Accepted,
	}

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

		c.ErrorChallenge(ctx, err)
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

	logging.Logger.Info("[challenge]validate: ",
		zap.Any("challenge_id", c.ChallengeID),
		zap.Time("created", createdTime))

	logging.Logger.Info("[challenge]elapsed:validate ",
		zap.String("challenge_id", c.ChallengeID),
		zap.Time("created", createdTime),
		zap.Time("start", startTime),
		zap.String("delay", startTime.Sub(createdTime).String()),
		zap.String("save", time.Since(startTime).String()))
}

func commitOnChain(id string) {

	startTime := time.Now()

	ctx := datastore.GetStore().CreateTransaction(context.TODO())
	defer ctx.Done()

	tx := datastore.GetStore().GetTransaction(ctx)

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

		c.ErrorChallenge(ctx, err)
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
	)

	go cMap.Delete(c.ChallengeID) //nolint

}

func loadTodoChallenges() {
	db := datastore.GetStore().GetDB()

	rows, err := db.Model(&ChallengeEntity{}).
		Where("status in (?,?)", Accepted, Processed).
		Order("created_at").
		Select("challenge_id", "created_at", "status").Rows()

	if err != nil {
		logging.Logger.Error("[challenge]todo",
			zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {

		var challengeID string
		var createdAt common.Timestamp
		var status ChallengeStatus

		err := rows.Scan(&challengeID, &createdAt, &status)
		if err != nil {
			logging.Logger.Error("[challenge]todo",
				zap.Error(err))
			continue
		}

		logging.Logger.Info("[challenge]todo",
			zap.String("challenge_id", challengeID),
			zap.String("status", status.String()),
			zap.Time("created_at", common.ToTime(createdAt)))

		nextTodoChallenge <- TodoChallenge{
			Id:        challengeID,
			CreatedAt: common.ToTime(createdAt),
			Status:    status,
		}

	}

}
