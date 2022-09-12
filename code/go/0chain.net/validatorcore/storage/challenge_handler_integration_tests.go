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

func ChallengeHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	state := conductrpc.Client().State()

	if state.AdversarialValidator.ID == node.Self.ID && state.AdversarialValidator.FailValidChallenge {
		challengeRequest, _, err := NewChallengeRequest(r)
		if err != nil {
			return nil, err
		}

		challengeObj, err := NewChallengeObj(ctx, challengeRequest)
		if err != nil {
			return nil, err
		}

		if len(challengeObj.Validators) > 2 {
			return InvalidValidationTicket(challengeObj, fmt.Errorf("Challenge failed by adversarial validator"))
		}
	}

	return challengeHandler(ctx, r)
}
