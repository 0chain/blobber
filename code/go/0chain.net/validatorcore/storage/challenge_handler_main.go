//go:build !integration_tests
// +build !integration_tests

package storage

import (
	"context"
	"net/http"
	"net/http/httputil"
)

func ChallengeHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	requestDump, err := httputil.DumpRequest(r, true)
	if err != nil {
		return nil, err
	}

	if len(Last5Transactions) >= 5 {
		Last5Transactions = Last5Transactions[1:]
	}
	Last5Transactions = append(Last5Transactions, string(requestDump))

	return challengeHandler(ctx, r)
}
