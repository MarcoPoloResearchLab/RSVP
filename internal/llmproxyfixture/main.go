// Package main supplies the deterministic LLM Proxy boundary for the localhost stack.
package main

import (
	"encoding/json"
	"github.com/tyemirov/RSVP/pkg/server"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/tyemirov/RSVP/pkg/services"
)

const (
	fixtureAddress   = "0.0.0.0:8082"
	fixtureParsePath = "/v2"
)

func main() {
	httpServer := server.NewHTTPServer(fixtureAddress, newHandler())
	log.Printf("Local LLM Proxy fixture listens on %s", fixtureAddress)
	if serveError := httpServer.ListenAndServe(); serveError != nil {
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
		if request.URL.Query().Get("key") != os.Getenv("LLM_PROXY_SECRET") || os.Getenv("LLM_PROXY_SECRET") == "" {
			http.Error(responseWriter, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		var messages struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Model           string `json:"model"`
			ReasoningEffort string `json:"reasoning_effort"`
		}
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if decodeError := decoder.Decode(&messages); decodeError != nil || len(messages.Messages) != 2 {
			http.Error(responseWriter, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		var input services.NaturalLanguageParseRequest
		if err := json.Unmarshal([]byte(messages.Messages[1].Content), &input); err != nil || strings.TrimSpace(input.InputText) == "" || input.ReferenceTime.IsZero() || input.Timezone == "" {
			http.Error(responseWriter, "Invalid temporal context", http.StatusBadRequest)
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
