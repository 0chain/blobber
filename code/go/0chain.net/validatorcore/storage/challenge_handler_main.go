//go:build !integration_tests
// +build !integration_tests

package storage

import (
	"context"
	"net/http"
)

func ChallengeHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	return challengeHandler(ctx, r)
}
