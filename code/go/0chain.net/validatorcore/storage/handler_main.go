//go:build !integration_tests
// +build !integration_tests

package storage

import (
	"github.com/gorilla/mux"
)

/*SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	setupHandlers(r)
}
