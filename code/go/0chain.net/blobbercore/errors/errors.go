package errors

import (
	"0chain.net/core/common"
)

var (
	//DBOpenError - Error opening the db
	DBOpenError = common.NewError("db_open_error", "Error opening the DB connection")
)
