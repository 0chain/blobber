package handler

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/challenge"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"net/http"
	"strconv"
)

func GetChallengeTimings(ctx context.Context, r *http.Request) (interface{}, error) {
	var (
		fromString = r.URL.Query().Get("from")
		from       common.Timestamp
	)

	if fromString != "" {
		fromI, err := strconv.Atoi(fromString)
		if err != nil {
			return nil, common.NewError("invalid_parameters", "from parameter is not valid")
		}
		from = common.Timestamp(fromI)
	}

	limit, err := common.GetOffsetLimitOrderParam(r.URL.Query())
	if err != nil {
		return nil, err
	}

	return challenge.GetChallengeTimings(from, limit)
}

func GetChallengeTiming(ctx context.Context, r *http.Request) (interface{}, error) {
	var (
		challengeID = r.URL.Query().Get("challenge_id")
	)

	return challenge.GetChallengeTiming(challengeID)
}
