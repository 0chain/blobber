package main

import (
	"net/http"

	"0chain.net/blobber"
)

func main() {

	blobber.SetupHandlers()
	http.ListenAndServe(":5050", nil)
}
