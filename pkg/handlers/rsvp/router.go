package rsvp

import (
	"github.com/temirov/RSVP/pkg/handlers/response"
	"net/http"
	"strings"

	"github.com/temirov/RSVP/pkg/config"
)

// Subrouter dispatches requests for paths starting with WebUnderRSVPs.
// It examines the remaining path to determine which handler to invoke.
func Subrouter(applicationContext *config.ApplicationContext) func(http.ResponseWriter, *http.Request) {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		remainingPath := strings.TrimPrefix(request.URL.Path, config.WebUnderRSVPs)
		if remainingPath == "" || remainingPath == "/" {
			http.NotFound(responseWriter, request)
			return
		}
		// Check for QR suffix.
		if strings.HasSuffix(remainingPath, config.WebQRSuffix) {
			extractedCode := strings.TrimSuffix(remainingPath, config.WebQRSuffix)
			extractedCode = strings.Trim(extractedCode, "/")
			Show(applicationContext, extractedCode).ServeHTTP(responseWriter, request)
			return
		}
		// Check for Thank You suffix.
		if strings.HasSuffix(remainingPath, config.WebThankYou) {
			extractedCode := strings.TrimSuffix(remainingPath, config.WebThankYou)
			extractedCode = strings.Trim(extractedCode, "/")
			response.Show(applicationContext, extractedCode).ServeHTTP(responseWriter, request)
			return
		}
		// Default: handle as single RSVP GET/POST.
		extractedCode := strings.Trim(remainingPath, "/")
		if request.Method == http.MethodGet {
			GetSingleRSVPHandler(applicationContext, extractedCode).ServeHTTP(responseWriter, request)
		} else if request.Method == http.MethodPost {
			Update(applicationContext, extractedCode).ServeHTTP(responseWriter, request)
		} else {
			http.Error(responseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	}
}
