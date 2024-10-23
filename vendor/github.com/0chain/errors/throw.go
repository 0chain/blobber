package errors

import (
	"strings"
)

// ApplicationError an appliction error with predifined error variable and detail message
type ApplicationError struct {
	// Inner inner error
	Inner error

	// MsgList detail message
	MsgList []string

	traceid string
}

// Error implement error.Error
func (e *ApplicationError) Error() string {
	return e.String()
}

// Error implement Stringer
func (e *ApplicationError) String() string {
	if e == nil {
		return ""
	}

	output := ""

	if e.traceid != "" {
		output = "[traceid-" + e.traceid + "] "
	}

	if e.Inner != nil && e.MsgList != nil {
		output += e.Inner.Error() + ": " + strings.Join(e.MsgList, "\r\n")
	} else if e.Inner != nil {
		output += e.Inner.Error()
	} else {
		output += strings.Join(e.MsgList, " ")
	}

	return output
}

// Unwrap implement error.Unwrap
func (e *ApplicationError) Unwrap() error {
	if e == nil {
		return nil
	}

	if e.Inner != nil {
		return e.Inner
	}

	return e
}

// Throw create an application error with prefinded error variable and message
// example
//    errors.Throw(ErrInvalidParameter, "bloober_id is missing")
func Throw(inner error, msgList ...string) error {
	return &ApplicationError{
		Inner:   inner,
		MsgList: msgList,
	}
}

// ThrowLog create an application error with prefinded error variable and message, log critical error in logging system for debugging.
// example
//    errors.ThrowLog(err.Error(), ErrInvalidParameter, "bloober_id is missing")
func ThrowLog(raw string, inner error, msgList ...string) error {

	callerName := getCallerName(1)

	traceid := generateTraceID(callerName, raw)

	return &ApplicationError{
		traceid: traceid,
		Inner:   inner,
		MsgList: msgList,
	}

}
