//go:build integration_tests
// +build integration_tests

package storage

import (
	"context"
	"fmt"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/conductor/conductrpc"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
)

func ChallengeHandler(ctx context.Context, r *http.Request) (interface{}, error, bool) {
	state := conductrpc.Client().State()

	if state.AdversarialValidator.ID == node.Self.ID && state.AdversarialValidator.FailValidChallenge {
		challengeRequest, _, err := NewChallengeRequest(r)
		if err != nil {
			return nil, err, true
		}

		challengeObj, err := NewChallengeObj(ctx, challengeRequest)
		if err != nil {
			return nil, err, true
		}

		if len(challengeObj.Validators) > 2 {
			res, err := InvalidValidationTicket(challengeObj, fmt.Errorf("Challenge failed by adversarial validator"))
			return res, err, true
		}
	} else if state.AdversarialValidator.ID == node.Self.ID && state.AdversarialValidator.DenialOfService {
		return nil, nil, false
	} else if state.AdversarialValidator.ID == node.Self.ID && state.AdversarialValidator.PassAllChallenges {
		challengeRequest, challengeHash, err := NewChallengeRequest(r)
		if err != nil {
			return nil, err, true
		}

		challengeObj, err := NewChallengeObj(ctx, challengeRequest)
		if err != nil {
			return nil, err, true
		}

		res, err := ValidValidationTicket(challengeObj, challengeRequest.ChallengeID, challengeHash)
		return res, err, true
	}

	res, err := challengeHandler(ctx, r)

	if state.NotifyOnValidationTicketGeneration {
		conductrpc.Client().ValidatorTicket(conductrpc.ValidtorTicket{
			ValidatorId: node.Self.ID,
		})
	}

	return res, err, true
}
