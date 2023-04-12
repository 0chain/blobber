package challenge

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"go.uber.org/zap"
)

type BCChallengeResponse struct {
	BlobberID  string             `json:"blobber_id"`
	Challenges []*ChallengeEntity `json:"challenges"`
}

const (
	challengeRequestInterval = time.Second * 30
	challengeMaxQueryLimit   = 50
	challengeOrder           = "desc"
)

var (
	unProcessedChallengeCh chan *ChallengeEntity
)

func init() {
	unProcessedChallengeCh = make(chan *ChallengeEntity, 1)
}

// syncOpenChallenges will request blockchain if it has been challenged and will receive
// maximum of 50 challenges in each request. It uses request params to request challenges
// namely blobberID, limit, from(challenges greater than `from` date) and sort(which should)
// be descending order.
func syncOpenChallenges(ctx context.Context) {
	var lastTimeStamp int64
	ticker := time.NewTicker(challengeRequestInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			params := map[string]string{
				"blobber": node.Self.ID,
				"limit":   strconv.Itoa(challengeMaxQueryLimit),
				"from":    strconv.FormatInt(lastTimeStamp, 10),
				"sort":    challengeOrder,
			}

			retBytes, err := transaction.MakeSCRestAPICall(
				transaction.STORAGE_CONTRACT_ADDRESS, "/openchallenges", params, chain.GetServerChain())
			if err != nil {
				logging.Logger.Error("[challenge]open: ", zap.Error(err))
				continue
			}

			var challengeResponse BCChallengeResponse
			if err := json.Unmarshal(retBytes, &challengeResponse); err != nil {
				logging.Logger.Error("[challenge]json: ", zap.Error(err))
				continue
			}

			logging.Logger.Info("Got new challenges", zap.Int("count", len(challengeResponse.Challenges)))
			for _, chal := range challengeResponse.Challenges {
				chal.StatusCh = make(chan ChallengeStatus, 1)
				chal.ErrCh = make(chan error)
				chal.ChallengeTiming = &ChallengeTiming{
					ChallengeID:      chal.ChallengeID,
					CreatedAtChain:   chal.CreatedAt,
					CreatedAtBlobber: common.Now(),
				}
				logging.Logger.Info("Sending challenge in channel to process.", zap.String("challenge_id", chal.ChallengeID))
				unProcessedChallengeCh <- chal
			}

			if len(challengeResponse.Challenges) > 0 {
				// update with last challenge's CreatedAt in the slice
				lastTimeStamp = int64(challengeResponse.Challenges[len(challengeResponse.Challenges)-1].CreatedAt)
			}
		}
	}
}

func ProcessChallenge(ctx context.Context) {
	guideCh := make(chan struct{}, config.Configuration.ChallengeNumWorkers)
	for chalEntity := range unProcessedChallengeCh {
		select {
		case <-ctx.Done():
			return
		default:
		}

		guideCh <- struct{}{}

		seqManagerCh <- chalEntity
		go func(chalEntity *ChallengeEntity) {
			logging.Logger.Info("Processing challenge", zap.String("challenge_id", chalEntity.ChallengeID))
			ctx := datastore.GetStore().CreateTransaction(ctx)
			defer func() {
				logging.Logger.Info("Saving challenge entity and challenge timing to database")
				chalEntity.UpdatedAt = time.Now().UTC()

				if err := chalEntity.Save(ctx); err != nil {
					logging.Logger.Error(err.Error())
				}

				if err := chalEntity.ChallengeTiming.Save(); err != nil {
					logging.Logger.Error(err.Error())
				}

				<-guideCh
			}()

			var err error
			t := common.ToTime(chalEntity.CreatedAt)
			if time.Since(t) > config.StorageSCConfig.ChallengeCompletionTime {
				updateChallengeStatus(chalEntity, Cancelled, "expired challenge")
				return
			}

			err = chalEntity.LoadValidationTickets(ctx)
			if err != nil {
				updateChallengeStatus(chalEntity, Cancelled, err.Error())
				return
			}
			chalEntity.ChallengeTiming.CompleteValidation = common.Now()

			chalEntity.StatusCh <- Completed
			err = <-chalEntity.ErrCh
			if err != nil {
				logging.Logger.Error("Challenge commit error", zap.Error(err))
				updateChallengeStatus(chalEntity, Cancelled, "Completed but cancelled due to error: "+err.Error())
				return
			}
			chalEntity.Status = Committed

		}(chalEntity)
	}
}

func updateChallengeStatus(chalEntity *ChallengeEntity, status ChallengeStatus, statusMessage string) {
	chalEntity.StatusCh <- status
	chalEntity.Status = status
	chalEntity.StatusMessage = statusMessage
}
