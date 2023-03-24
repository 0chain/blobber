package challenge

import (
	"context"
	"encoding/json"
	"fmt"
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
	// offset := 0
	challengeRespTime := time.Now()
	for {
		d := time.Since(challengeRespTime)
		cri := challengeRequestInterval
		if d > cri {
			cri = 0
		} else {
			cri = cri - d
		}
		<-time.After(cri)

		params := map[string]string{
			"blobber": node.Self.ID,
			"limit":   strconv.Itoa(challengeMaxQueryLimit),
			"from":    strconv.FormatInt(lastTimeStamp, 10),
			// should avoid use of offset because `from` parameter has already filtered the challenges
			// "offset":  strconv.Itoa(offset),
			"sort": challengeOrder,
		}

		retBytes, err := transaction.MakeSCRestAPICall(
			transaction.STORAGE_CONTRACT_ADDRESS, "/openchallenges", params, chain.GetServerChain())
		if err != nil {
			logging.Logger.Error("[challenge]open: ", zap.Error(err))
			break
		}
		challengeRespTime = time.Now()

		var challengeResponse BCChallengeResponse
		if err := json.Unmarshal(retBytes, &challengeResponse); err != nil {
			logging.Logger.Error("[challenge]json: ", zap.Error(err))
			break
		}

		logging.Logger.Info(fmt.Sprintf("Got %d new challenges", len(challengeResponse.Challenges)))
		for _, chal := range challengeResponse.Challenges {
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
		// offset += len(challengeResponse.Challenges)
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

		go func(chalEntity *ChallengeEntity) {
			defer func() {
				<-guideCh
			}()
			logging.Logger.Info("Processing challenge", zap.String("challenge_id", chalEntity.ChallengeID))
			ctx := datastore.GetStore().CreateTransaction(context.TODO())
			defer func() {
				if err := chalEntity.Save(ctx); err != nil {
					logging.Logger.Error(err.Error())
				}

				if err := chalEntity.ChallengeTiming.Save(); err != nil {
					logging.Logger.Error(err.Error())
				}
			}()

			var err error
			t := common.ToTime(chalEntity.CreatedAt)
			if time.Since(t) > config.StorageSCConfig.ChallengeCompletionTime {
				chalEntity.Status = Cancelled
				chalEntity.StatusMessage = "expired challenge"
				return
			}

			err = chalEntity.LoadValidationTickets(ctx)
			if err != nil {
				chalEntity.Status = Cancelled
				chalEntity.StatusMessage = err.Error()
				return
			}
			chalEntity.ChallengeTiming.CompleteValidation = common.Now()

			err = chalEntity.CommitChallenge(ctx)
			if err != nil {
				chalEntity.Status = Cancelled
				chalEntity.StatusMessage = err.Error()
				chalEntity.UpdatedAt = time.Now().UTC()
				return
			}
			chalEntity.UpdatedAt = time.Now().UTC()
			chalEntity.Status = Committed

		}(chalEntity)

	}
}
