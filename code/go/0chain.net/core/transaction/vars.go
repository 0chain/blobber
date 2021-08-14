package transaction

import (
	"errors"
	"time"
)

var (
	// DefaultDialTimeout default timeout of a dialer
	DefaultDialTimeout = 5 * time.Second
	// DefaultRequestTimeout default time out of a http request
	DefaultRequestTimeout = 10 * time.Second
	// DefaultRetry retry times if a request is failed with 5xx status code
	DefaultRetry = 3

	// MinConfirmation minial confirmation from sharders
	MinConfirmation = 50
)

var (
	// ErrBadRequest bad request from sharder
	ErrBadRequest = errors.New("[sharder]bad request")

	// ErrNoAvailableSharder no any available sharder
	ErrNoAvailableSharder = errors.New("[txn] there is no any available sharder")

	// ErrTooLessConfirmation too less sharder to confirm transaction
	ErrTooLessConfirmation = errors.New("[txn] too less sharders to confirm it")
)
