package naturallanguage_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/providers/naturallanguage"
	"github.com/tyemirov/RSVP/pkg/services"
)

func TestLLMProxyOfficialClientTemporalContract(t *testing.T) {
	reference := time.Date(2030, 1, 2, 15, 0, 0, 0, time.UTC)
	provider, model := "fixture-provider", "fixture-model"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != "POST" || request.URL.Path != "/v2" || request.URL.Query().Get("key") != "fixture-secret" || request.URL.Query().Get("provider") != provider || request.URL.Query().Get("format") != "text/plain" {
			t.Error("official client path or authentication did not match the profile")
		}
		if request.Header.Get("X-LLM-Proxy-Request-Timeout-Seconds") != "123" {
			t.Error("configured work budget did not reach the official header")
		}
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Model           string `json:"model"`
			ReasoningEffort string `json:"reasoning_effort"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if body.Model != model || body.ReasoningEffort != "low" || len(body.Messages) != 2 || body.Messages[0].Role != "system" || body.Messages[1].Role != "user" {
			t.Error("configured model, reasoning, or messages did not reach the provider")
			return
		}
		var input services.NaturalLanguageParseRequest
		if err := json.Unmarshal([]byte(body.Messages[1].Content), &input); err != nil {
			t.Error(err)
			return
		}
		if input.InputText != "waiting request" || !input.ReferenceTime.Equal(reference) || input.Timezone != "America/Los_Angeles" {
			t.Error("explicit temporal context did not reach the provider")
		}
		_, _ = fmt.Fprint(writer, `{"mode":"open_lane","title":"Waiting","anchor_event_id":null,"starts_at":null,"ends_at":null,"review_interval_seconds":null,"next_probe_at":null,"escalation_interval_seconds":null,"relative_rules":[]}`)
	}))
	defer server.Close()
	adapter, err := naturallanguage.New(config.LLMProxyConfig{BaseURL: server.URL, Secret: "fixture-secret", Provider: &provider, Model: &model, ReasoningEffort: "low", RequestTimeoutSeconds: 123}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Parse(context.Background(), services.NaturalLanguageParseRequest{InputText: "waiting request", ReferenceTime: reference, Timezone: "America/Los_Angeles"})
	if err != nil || result.Mode != "open_lane" || result.Title != "Waiting" {
		t.Fatalf("parse result = %#v, error = %v", result, err)
	}
}

func TestLLMProxyFailuresDoNotExposePrivateInputs(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		status   int
		body     string
		expected error
	}{
		{"invalid schema", 200, `{"secret_provider_value":"private-input"}`, services.ErrNaturalLanguageProviderResponseInvalid},
		{"provider failure", 502, "private-input", services.ErrNaturalLanguageProviderFailed},
		{"trailing document", 200, `{"mode":"open_lane"} {}`, services.ErrNaturalLanguageProviderResponseInvalid},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(scenario.status)
				_, _ = fmt.Fprint(writer, scenario.body)
			}))
			defer server.Close()
			empty := ""
			adapter, err := naturallanguage.New(config.LLMProxyConfig{BaseURL: server.URL, Secret: "private-input", Provider: &empty, Model: &empty, ReasoningEffort: "low", RequestTimeoutSeconds: 60}, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			_, err = adapter.Parse(context.Background(), services.NaturalLanguageParseRequest{InputText: "private-input", ReferenceTime: time.Now(), Timezone: "UTC"})
			if !errors.Is(err, scenario.expected) || strings.Contains(err.Error(), "private-input") {
				t.Errorf("private-safe error contract failed")
			}
		})
	}
}
