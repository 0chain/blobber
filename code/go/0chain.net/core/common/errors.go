package common

import (
	"fmt"
)

/*Error type for a new application error */
type Error struct {
	Code       string `json:"code,omitempty"`
	Msg        string `json:"msg"`
	StatusCode int    `json:"status_code,omitempty"`
}

func (err *Error) Error() string {
	return fmt.Sprintf("%s: %s", err.Code, err.Msg)
}

/*NewError - create a new error */
func NewError(code, msg string) *Error {
	return &Error{Code: code, Msg: msg}
}

/*NewErrorf - create a new error with format */
func NewErrorf(code, format string, args ...interface{}) *Error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, args...)}
}

/*NewErrorf - create a new error with format */
func NewErrorfWithStatusCode(statusCode int, errCode, format string, args ...interface{}) *Error {
	return &Error{StatusCode: statusCode, Code: errCode, Msg: fmt.Sprintf(format, args...)}
}

/*InvalidRequest - create error messages that are needed when validating request input */
func InvalidRequest(msg string) error {
	return NewError("invalid_request", fmt.Sprintf("Invalid request (%v)", msg))
}
