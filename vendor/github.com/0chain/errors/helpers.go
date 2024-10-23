// Package errors - helpers
package errors

// Top since errors can be wrapped and stacked,
// it's necessary to get the top level error for tests and validations
func Top(err error) string {
	if err == nil {
		return ""
	}
	current, _ := UnWrap(err)
	return current.Error()
}

// Cause returns the underlying cause of the error
func Cause(err error) error {
	var current, previous error
	for {
		current, previous = UnWrap(err)
		if previous == nil {
			return current
		}
	}
}
