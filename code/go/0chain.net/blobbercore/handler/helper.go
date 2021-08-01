package handler

import (
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

func checkValidDate(s string) error {
	if s != "" {
		_, err := time.Parse("2006-01-02 15:04:05.999999999", s)
		if err != nil {
			return common.NewError("invalid_parameters", err.Error())
		}
	}
	return nil
}
