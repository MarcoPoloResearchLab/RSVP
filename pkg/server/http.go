// Package server constructs HTTP listeners for RSVP applications.
package server

import (
	"net/http"
	"time"
)

const readHeaderTimeout = 5 * time.Second

// NewHTTPServer constructs a server for the supplied address and handler.
func NewHTTPServer(address string, handler http.Handler) *http.Server {
	return &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: readHeaderTimeout}
}
