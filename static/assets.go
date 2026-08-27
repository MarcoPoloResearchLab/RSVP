// Package staticassets supplies browser assets through the application binary.
package staticassets

import (
	"embed"
	"net/http"
)

//go:embed horizon.css horizon.js
var files embed.FS

// Handler returns the embedded static asset handler.
func Handler() http.Handler {
	return http.FileServer(http.FS(files))
}
