package transaction

import (
	"errors"
)

var (

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
