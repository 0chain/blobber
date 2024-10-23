// Package errors - Is
package errors

import "errors"

/*Is - tells whether actual error is targer error
where, actual error can be either Error/withError
if actual error is wrapped error then if any internal error
matches the target error then function results in true*/
func Is(actual error, target error) bool {

	if errors.Is(actual, target) {
		return true
	}

	switch targetError := target.(type) {
	case *Error:
		switch actualError := actual.(type) {
		case *Error:
			if actualError.Code == "" && targetError.Code == "" {
				return actualError.Msg == targetError.Msg
			}

			return actualError.Code == targetError.Code

		case *withError:
			return Is(actualError.current, target) || Is(actualError.previous, target)
		default:
			return false
		}
	default:
		return false
	}
}

// As wrap errors.As
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// Unwrap wrap errors.Unwrap
func Unwrap(err error) error {
	return errors.Unwrap(err)
}
