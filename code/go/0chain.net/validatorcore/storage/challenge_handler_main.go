//go:build !integration_tests
// +build !integration_tests

package storage

import (
	"context"
	"net/http"
)

func ChallengeHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	res, err := challengeHandler(ctx, r)

	if len(Last5Transactions) >= 5 {
		Last5Transactions = Last5Transactions[1:]
	}
	Last5Transactions = append(Last5Transactions, res)

	return res, err
}
