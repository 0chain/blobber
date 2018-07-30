package main

import (
	"net/http"

	"0chain.net/blobber"
	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()
	blobber.SetupHandlers(r)
	http.ListenAndServe(":5050", r)
}
