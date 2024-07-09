package handler

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/challenge"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"net/http"
	"strconv"
)

// swagger:route GET /challengetimings GetChallengeTimings
// Get challenge timings.
//
// Retrieve challenge timings for the blobber admin.
//
// parameters:
//
//   +name: Authorization
//     in: header
//     type: string
//     required: true
//     description: Authorization header (Basic auth). MUST be provided to fulfil the request
//   +name: from
//     in: query
//     type: integer
//     required: false
//     description: An optional timestamp from which to retrieve the challenge timings
//   +name: offset
//     in: query
//     type: integer
//     required: false
//     description: Pagination offset, start of the page to retrieve. Default is 0.
//   +name: limit
//     in: query
//     type: integer
//     required: false
//     description: Pagination limit, number of entries in the page to retrieve. Default is 20.
//   +name: sort
//     in: query
//     type: string
//     required: false
//     description: Direction of sorting based on challenge closure time, either "asc" or "desc". Default is "asc"
//
// responses:
//   200: []ChallengeTiming
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

// swagger:route GET /challenge-timings-by-challengeId GetChallengeTimingByChallengeID
// Get challenge timing by challenge ID.
// Retrieve challenge timing for the given challenge ID by the blobber admin.
//
// parameters:
//
//   +name: Authorization
//     in: header
//     type: string
//     required: true
//     description: Authorization header (Basic auth). MUST be provided to fulfil the request
//   +name: challenge_id
//     in: query
//     type: string
//     required: true
//     description: Challenge ID for which to retrieve the challenge timing
//
// responses:
//   200: ChallengeTiming
func GetChallengeTiming(ctx context.Context, r *http.Request) (interface{}, error) {
	var (
		challengeID = r.URL.Query().Get("challenge_id")
	)

	return challenge.GetChallengeTiming(challengeID)
}
