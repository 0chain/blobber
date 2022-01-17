// Package dev providers tools for local development
package dev

import (
	"net/http/httptest"

	"github.com/0chain/blobber/code/go/0chain.net/dev/miner"
	"github.com/0chain/blobber/code/go/0chain.net/dev/sharder"
	"github.com/gorilla/mux"
)

// Server a local dev server to mock server APIs
type Server struct {
	*httptest.Server
	*mux.Router
}

// NewServer create a local dev server
func NewServer() *Server {
	router := mux.NewRouter()
	s := &Server{
		Router: router,
		Server: httptest.NewServer(router),
	}

	return s
}

// NewSharderServer create a local dev sharder server
func NewSharderServer() *Server {
	s := NewServer()

	sharder.RegisterHandlers(s.Router)

	return s
}

// NewMinerServer create a local dev miner server
func NewMinerServer() *Server {
	s := NewServer()

	miner.RegisterHandlers(s.Router)

	return s
}
