package routes_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/routes"
)

func TestNaturalLanguageIngestionStopsStalledProxy(t *testing.T) {
	for _, stage := range []string{"headers", "body"} {
		t.Run(stage, func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				_, _ = io.Copy(io.Discard, request.Body)
				if stage == "body" {
					writer.Header().Set("Content-Type", "text/plain")
					_, _ = writer.Write([]byte(`{"mode":`))
					writer.(http.Flusher).Flush()
				}
				<-request.Context().Done()
			}))
			defer provider.Close()
			fixture := testsupport.NewFixture(t)
			owner := fixture.CreateUser(testsupport.OwnerUserID)
			event := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
			var lane models.Lane
			if err := fixture.Database.First(&lane, "id = ?", event.LaneID).Error; err != nil {
				t.Fatal(err)
			}
			configuration := testsupport.EnvironmentConfig()
			configuration.LLMProxy.BaseURL = provider.URL
			configuration.LLMProxy.RequestTimeoutSeconds = 1
			application, err := routes.New(fixture.ApplicationContext, configuration)
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			application.RegisterRoutes(mux)
			server := httptest.NewServer(mux)
			defer server.Close()
			body := fmt.Sprintf(`{"source":"natural_language","input_text":"wait for a decision","calendar_id":%q,"reference_time":%q,"timezone":%q}`, lane.CalendarID, testsupport.FixedStartTime().Format(time.RFC3339Nano), testsupport.TimezoneName)
			request, err := http.NewRequest(http.MethodPost, server.URL+config.WebIngestionDrafts, strings.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(testsupport.SessionCookie(t, owner))
			client := server.Client()
			// The caller watchdog exceeds the application's configured provider budget.
			client.Timeout = 3 * time.Second
			response, err := client.Do(request)
			if err != nil {
				t.Fatalf("ingestion did not stop the stalled proxy within its budget: %v", err)
			}
			defer response.Body.Close()
			payload, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != http.StatusBadGateway || !strings.Contains(string(payload), "natural_language_provider_failed") {
				t.Fatalf("stalled proxy response: status=%d body=%s", response.StatusCode, payload)
			}
			var drafts int64
			if err := fixture.Database.Model(&models.IngestionDraft{}).Count(&drafts).Error; err != nil {
				t.Fatal(err)
			}
			if drafts != 0 {
				t.Fatalf("stalled provider persisted %d drafts", drafts)
			}
		})
	}
}
