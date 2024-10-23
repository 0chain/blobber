// Package errors - application error interface implementation
package errors

import (
	"fmt"
	"strings"
)

/*Error type for a new application error */
type Error struct {
	Code string `json:"code,omitempty"`
	Msg  string `json:"msg"`
}

func (err *Error) Error() string {
	if err.Code == "" {
		return err.Msg
	}
	return fmt.Sprintf("%s: %s", err.Code, err.Msg)
}

// New - create a new error
func New(code, msg string) *Error {
	return new(code, msg)
}

// Newf - creates a new error
func Newf(code, format string, args ...interface{}) *Error {
	return new(code, fmt.Sprintf(format, args...))
}

func new(code, msg string) *Error {
	currentError := Error{
		Code: strings.TrimSpace(code),
		Msg:  strings.TrimSpace(msg),
	}

	return &currentError
}
