package naturallanguage

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/services"
)

func TestAdapterAuthenticatesAndUsesExplicitTemporalContext(testingContext *testing.T) {
	referenceTime := time.Date(2030, 1, 2, 15, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer private-key" {
			testingContext.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		var body services.NaturalLanguageParseRequest
		if decodeError := json.NewDecoder(request.Body).Decode(&body); decodeError != nil {
			testingContext.Errorf("decode request: %v", decodeError)
		}
		if body.InputText != "waiting request" || !body.ReferenceTime.Equal(referenceTime) || body.Timezone != "America/Los_Angeles" {
			testingContext.Errorf("request body = %#v", body)
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"mode":"open_lane","title":"Waiting","anchor_event_id":null,"starts_at":null,"ends_at":null,"review_interval_seconds":null,"next_probe_at":null,"escalation_interval_seconds":null,"relative_rules":[]}`))
	}))
	defer server.Close()
	adapter, adapterError := New(server.URL, "private-key", server.Client())
	if adapterError != nil {
		testingContext.Fatalf("construct adapter: %v", adapterError)
	}
	response, parseError := adapter.Parse(context.Background(), services.NaturalLanguageParseRequest{InputText: "waiting request", ReferenceTime: referenceTime, Timezone: "America/Los_Angeles"})
	if parseError != nil || response.Mode != models.IngestionModeOpenLane {
		testingContext.Fatalf("response = %#v, error = %v", response, parseError)
	}
}

func TestAdapterRejectsUnknownProviderFieldsWithoutLeakingPayload(testingContext *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		_, _ = responseWriter.Write([]byte(`{"mode":"open_lane","secret_provider_value":"must-not-leak"}`))
	}))
	defer server.Close()
	adapter, _ := New(server.URL, "private-key", server.Client())
	_, parseError := adapter.Parse(context.Background(), services.NaturalLanguageParseRequest{InputText: "private input", ReferenceTime: time.Now(), Timezone: "America/Los_Angeles"})
	if !errors.Is(parseError, services.ErrNaturalLanguageProviderResponseInvalid) {
		testingContext.Fatalf("unknown field error = %v", parseError)
	}
	if parseError.Error() != services.ErrNaturalLanguageProviderResponseInvalid.Error() {
		testingContext.Fatalf("provider error leaked data: %q", parseError)
	}
}
