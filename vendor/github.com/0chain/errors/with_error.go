// Package errors - withError implementing error interface
package errors

type withError struct {
	previous error
	current  error
}

func (w *withError) Error() string {
	var retError string
	if w.current != nil {
		retError = w.current.Error()
	}
	if w.previous != nil {
		retError += "\n" + w.previous.Error()
	}
	return retError
}
