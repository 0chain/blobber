package main

import (
	"net/http"

	"0chain.net/blobber"
	"0chain.net/config"
	"0chain.net/logging"
	"github.com/gorilla/mux"
)

func main() {
	config.SetupConfig()
	logging.InitLogging("development")
	r := mux.NewRouter()
	blobber.SetupHandlers(r)
	http.ListenAndServe(":5050", r)
}
