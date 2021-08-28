package handler

import (
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

func checkValidDate(s, dateLayOut string) error {
	if s != "" {
		_, err := time.Parse(dateLayOut, s)
		if err != nil {
			return common.NewError("invalid_parameters", err.Error())
		}
	}
	return nil
}
