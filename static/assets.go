// Package staticassets supplies browser assets through the application binary.
package staticassets

import (
	"embed"
	"net/http"
)

//go:embed index.html shell.js app.css horizon.css horizon.js settings.css settings.js
var files embed.FS

// Handler returns the embedded static asset handler.
func Handler() http.Handler {
	return http.FileServer(http.FS(files))
}
