// Package main supplies the deterministic parser for the localhost stack.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/tyemirov/RSVP/pkg/services"
)

const (
	fixtureAddress   = "0.0.0.0:8082"
	fixtureAPIKey    = "rsvp-local-parser"
	fixtureParsePath = "/parse"
)

func main() {
	server := &http.Server{Addr: fixtureAddress, Handler: newHandler()}
	log.Printf("Local natural-language parser listens on %s", fixtureAddress)
	if serveError := server.ListenAndServe(); serveError != nil {
		log.Fatal(serveError)
	}
}

func newHandler() http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != fixtureParsePath {
			http.NotFound(responseWriter, request)
			return
		}
		if request.Method != http.MethodPost {
			responseWriter.Header().Set("Allow", http.MethodPost)
			http.Error(responseWriter, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		if request.Header.Get("Authorization") != "Bearer "+fixtureAPIKey {
			http.Error(responseWriter, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		var input services.NaturalLanguageParseRequest
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if decodeError := decoder.Decode(&input); decodeError != nil || strings.TrimSpace(input.InputText) == "" || input.ReferenceTime.IsZero() || strings.TrimSpace(input.Timezone) == "" {
			http.Error(responseWriter, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		title := strings.TrimSpace(input.InputText)
		if len(title) > 200 {
			title = title[:200]
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(responseWriter).Encode(services.NaturalLanguageParseResponse{
			Mode:          "open_lane",
			Title:         title,
			RelativeRules: []services.NaturalLanguageRelativeRule{},
		})
	})
}
